//! First feasibility probe: render a single offline page into a bounded RGBA frame.
//! stdout is binary; all diagnostics go to stderr.
use std::cell::Cell;
use std::error::Error;
use std::io::{self, Write};
use std::rc::Rc;
use std::sync::mpsc::{self, SyncSender};
use std::time::{Duration, Instant};

use dpi::PhysicalSize;
use servo::{
    EventLoopWaker, LoadStatus, NavigationRequest, Preferences, RenderingContext, ServoBuilder,
    SoftwareRenderingContext, WebResourceLoad, WebResourceResponse, WebView, WebViewBuilder,
    WebViewDelegate,
};
use url::Url;
mod egl;
mod gl_profile;
mod session;

const MAX_PIXELS: u32 = 1280 * 720;
const TIMEOUT: Duration = Duration::from_secs(30);

fn profile_stage(stage: &str, started: Instant) {
    if std::env::var_os("KIWIX_PROFILE_DIR").is_none() {
        return;
    }
    let status = std::fs::read_to_string("/proc/self/status").unwrap_or_default();
    let memory = status
        .lines()
        .filter(|line| {
            ["VmRSS:", "VmHWM:", "VmSwap:", "VmSize:", "Threads:"]
                .iter()
                .any(|key| line.starts_with(key))
        })
        .collect::<Vec<_>>()
        .join("; ");
    eprintln!(
        "servo.profile pid={} stage={stage} elapsed_ms={} {memory}",
        std::process::id(),
        started.elapsed().as_millis()
    );
}

#[derive(Clone)]
struct Waker(SyncSender<()>);
impl EventLoopWaker for Waker {
    fn clone_box(&self) -> Box<dyn EventLoopWaker> {
        Box::new(self.clone())
    }
    fn wake(&self) {
        // Coalesce wakes rather than block a Servo internal thread.
        let _ = self.0.try_send(());
    }
}

struct Delegate {
    origin: Url,
    frame: Cell<bool>,
    navigation: Option<SyncSender<serde_json::Value>>,
    load_status: Cell<LoadStatus>,
}
impl WebViewDelegate for Delegate {
    fn notify_new_frame_ready(&self, _: WebView) {
        self.frame.set(true);
    }
    fn notify_load_status_changed(&self, webview: WebView, status: LoadStatus) {
        if webview
            .url()
            .is_some_and(|url| url.origin() == self.origin.origin())
        {
            self.load_status.set(status);
        }
    }
    fn request_navigation(&self, _: WebView, request: NavigationRequest) {
        if let Some(tx) = &self.navigation {
            if request.url == self.origin {
                request.allow();
                return;
            }
            let url = request.url.to_string();
            if url.len() <= 8192 {
                let _ = tx.try_send(serde_json::json!({"event":"navigate", "url":url}));
            }
            request.deny();
        } else if request.url.origin() == self.origin.origin() {
            request.allow();
        } else {
            request.deny();
        }
    }
    fn load_web_resource(&self, _: WebView, load: WebResourceLoad) {
        let resource_url = load.request().url.as_url();
        // Servo serializes inline <svg> into an internal data image. It has
        // an opaque origin, so the loopback-origin check below would cancel it.
        // Keep this exception limited to bounded SVG images, never documents.
        if !load.request().is_for_main_frame
            && resource_url
                .as_str()
                .starts_with("data:image/svg+xml;base64,")
            && resource_url.as_str().len() <= 8 * 1024 * 1024
        {
            return;
        }
        if load.request().url.as_url().origin() != self.origin.origin() {
            let response = WebResourceResponse::new(load.request().url.as_url().clone());
            load.intercept(response).cancel();
        }
    }
}

fn main() {
    if let Err(error) = run() {
        eprintln!("zimlite-servo-host: {error}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn Error>> {
    let process_started = Instant::now();
    profile_stage("process.start", process_started);
    let mut args: Vec<String> = std::env::args().collect();
    let persistent = args.get(1).is_some_and(|arg| arg == "--session");
    if persistent {
        args.remove(1);
    }
    if args.len() != 4 {
        return Err("usage: zimlite-servo-host [--session] URL WIDTH HEIGHT".into());
    }
    let url = Url::parse(&args[1])?;
    if url.scheme() != "http" || url.host_str() != Some("127.0.0.1") {
        return Err("probe accepts only a loopback HTTP source".into());
    }
    let width: u32 = args[2].parse()?;
    let height: u32 = args[3].parse()?;
    if width == 0
        || height == 0
        || width > 1280
        || height > 1280
        || width.checked_mul(height).is_none_or(|n| n > MAX_PIXELS)
    {
        return Err("viewport exceeds the probe pixel budget".into());
    }
    rustls::crypto::aws_lc_rs::default_provider()
        .install_default()
        .map_err(|_| "failed to install TLS provider")?;

    let size = PhysicalSize::new(width, height);
    if persistent {
        session::write_event(serde_json::json!({"event":"progress", "phase":"context"}))?;
    }
    profile_stage("context.begin", process_started);
    let context: Rc<dyn RenderingContext> = match std::env::var("ZIMLITE_SERVO_CONTEXT").as_deref()
    {
        Ok("auto") => match egl::EglContext::new(size) {
            Ok(context) => Rc::new(context),
            Err(error) => {
                eprintln!("firmware GLES unavailable: {error}; trying software Mesa");
                return restart_with_software();
            }
        },
        Ok("gles") => Rc::new(egl::EglContext::new(size)?),
        Ok("software") | Err(_) => Rc::new(
            SoftwareRenderingContext::new(size)
                .map_err(|error| format!("software context: {error:?}"))?,
        ),
        _ => return Err("unknown ZIMLITE_SERVO_CONTEXT (auto, software or gles)".into()),
    };
    context
        .make_current()
        .map_err(|error| format!("make context current: {error:?}"))?;
    profile_stage("context.ready", process_started);
    if persistent {
        session::write_event(serde_json::json!({"event":"progress", "phase":"engine"}))?;
    }
    let (tx, rx) = mpsc::sync_channel(1);
    let mut preferences = Preferences::default();
    preferences.layout_threads = 1;
    preferences.network_http_cache_disabled = true;
    preferences.js_mem_max = 128;
    preferences.thread_pool_workers_max = 2;
    preferences.thread_pool_async_runtime_workers_max = 2;
    preferences.thread_pool_fallback_workers = 1;
    preferences.thread_pool_canvas_workers = 0;
    preferences.thread_pool_webrender_workers_max = 2;
    if std::env::var("LIBGL_ALWAYS_SOFTWARE").as_deref() == Ok("1") {
        // Grayscale AA keeps text smooth without the extra subpixel passes
        // that are particularly expensive on a CPU rasterizer.
        preferences.gfx_subpixel_text_antialiasing_enabled = false;
    }
    preferences.session_history_max_length = 1;
    profile_stage("servo_builder.begin", process_started);
    let servo = ServoBuilder::default()
        .preferences(preferences)
        .event_loop_waker(Box::new(Waker(tx)))
        .build();
    profile_stage("servo_builder.ready", process_started);
    let (events_tx, events_rx) = mpsc::sync_channel(16);
    let delegate = Rc::new(Delegate {
        origin: url.clone(),
        frame: Cell::new(false),
        navigation: persistent.then(|| events_tx.clone()),
        load_status: Cell::new(LoadStatus::Started),
    });
    profile_stage("webview.begin", process_started);
    let webview = WebViewBuilder::new(&servo, context.clone())
        .url(url)
        .delegate(delegate.clone())
        .build();
    if let Ok(theme) = std::env::var("ZIMLITE_SERVO_THEME") {
        match theme.as_str() {
            "dark" => webview.notify_theme_change(servo::Theme::Dark),
            "light" => webview.notify_theme_change(servo::Theme::Light),
            _ => return Err("unknown initial theme".into()),
        }
    }
    if let Ok(scale) = std::env::var("ZIMLITE_SERVO_ZOOM") {
        let scale: f32 = scale.parse()?;
        if !scale.is_finite() || !(0.5..=2.0).contains(&scale) {
            return Err("initial zoom outside budget".into());
        }
        if scale != webview.page_zoom() {
            webview.set_page_zoom(scale);
        }
    }
    webview.show();
    profile_stage("webview.ready", process_started);
    if persistent {
        session::write_event(serde_json::json!({"event":"progress", "phase":"document"}))?;
        return session::run(
            &servo, &webview, context, &delegate, &rx, events_tx, events_rx,
        );
    }
    let (screenshot_tx, screenshot_rx) = mpsc::sync_channel(1);
    // The screenshot API waits for fonts, images, stylesheets and final layout.
    webview.take_screenshot(None, move |result| {
        let _ = screenshot_tx.try_send(result);
    });
    profile_stage("screenshot.requested", process_started);
    let started = Instant::now();
    loop {
        servo.spin_event_loop();
        if delegate.frame.replace(false) {
            webview.paint();
        }
        if let Ok(result) = screenshot_rx.try_recv() {
            profile_stage("screenshot.ready", process_started);
            let image = result.map_err(|error| format!("screenshot failed: {error:?}"))?;
            if image.width() != width || image.height() != height {
                return Err("screenshot dimensions do not match requested viewport".into());
            }
            let mut output = io::stdout().lock();
            // Fixed header avoids allocating/decoding an unbounded IPC message.
            output.write_all(b"ZSR1")?;
            output.write_all(&width.to_le_bytes())?;
            output.write_all(&height.to_le_bytes())?;
            output.write_all(image.as_raw())?;
            output.flush()?;
            drop(output);
            if let Ok(status) = std::fs::read_to_string("/proc/self/status") {
                for line in status.lines().filter(|line| line.starts_with("VmHWM:")) {
                    eprintln!("helper {line}");
                }
            }
            if let Ok(peak) = std::fs::read_to_string("/sys/fs/cgroup/memory.peak") {
                eprintln!("container memory peak: {} bytes", peak.trim());
            }
            eprintln!(
                "first complete frame: {} ms total; {} ms after WebView creation",
                process_started.elapsed().as_millis(),
                started.elapsed().as_millis()
            );
            return Ok(());
        }
        if started.elapsed() > TIMEOUT {
            return Err("timed out waiting for a complete frame".into());
        }
        let _ = rx.recv_timeout(Duration::from_millis(16));
    }
}

fn restart_with_software() -> Result<(), Box<dyn Error>> {
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        let directory = std::env::var("ZIMLITE_SERVO_SOFTWARE_LIB_DIR")
            .map_err(|_| "software Mesa runtime is not installed")?;
        let library_path = format!(
            "{directory}:{}",
            std::env::var("LD_LIBRARY_PATH").unwrap_or_default()
        );
        let error = std::process::Command::new(std::env::current_exe()?)
            .args(std::env::args_os().skip(1))
            .env("ZIMLITE_SERVO_CONTEXT", "gles")
            .env("LD_LIBRARY_PATH", library_path)
            .env("LIBGL_DRIVERS_PATH", format!("{directory}/dri"))
            .env(
                "__EGL_VENDOR_LIBRARY_FILENAMES",
                format!("{directory}/egl_vendor.json"),
            )
            .env("LIBGL_ALWAYS_SOFTWARE", "1")
            .env("GALLIUM_DRIVER", "llvmpipe")
            .env("LP_NUM_THREADS", "2")
            .env("EGL_PLATFORM", "surfaceless")
            .exec();
        Err(error.into())
    }
    #[cfg(not(unix))]
    Err("software restart requires Unix".into())
}
