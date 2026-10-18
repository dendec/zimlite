//! Bounded control messages and latest-viewport rendering for the application.
use std::cell::Cell;
use std::error::Error;
use std::io::{self, Read, Write};
use std::rc::Rc;
use std::sync::mpsc::{self, Receiver, SyncSender};
use std::time::{Duration, Instant};

use dpi::PhysicalSize;
use serde::Deserialize;
use serde_json::{Value, json};
use servo::{
    DeviceIntRect, DeviceIntSize, DevicePoint, JSValue, RenderingContext, Scroll, Servo, WebView,
    WebViewPoint, WebViewVector,
};

const MAX_COMMAND: usize = 8192;
const MAX_EVENT: usize = 65536;

// Selection belongs to the DOM so queued navigation uses current scroll and
// layout geometry, rather than an index predicted from an older state packet.
const LINK_STATE_JS: &str = r#"
    const links = document.querySelectorAll('a[href]');
    const n = Math.min(links.length, 2048);
    const selected = Array.prototype.indexOf.call(links,
      document.querySelector('[data-zimlite-selected]'));
"#;

const LINK_SELECTION_JS: &str = r#"
    function zimliteLinkVisible(a) {
      if (!a) return false;
      // Reject offscreen links before walking their clipping ancestors.
      const rects = Array.from(a.getClientRects()).filter(r =>
        r.width > 0 && r.height > 0 && r.bottom > 0 && r.top < innerHeight &&
        r.right > 0 && r.left < innerWidth);
      if (!rects.length || getComputedStyle(a).visibility !== 'visible') return false;
      let left = 0, top = 0, right = innerWidth, bottom = innerHeight;
      for (let p = a.parentElement; p; p = p.parentElement) {
        // Root scrolling is already represented by the viewport bounds.
        if (p === document.scrollingElement || p === document.documentElement) continue;
        const s = getComputedStyle(p);
        if (s.overflowX === 'visible' && s.overflowY === 'visible') continue;
        const r = p.getBoundingClientRect();
        if (s.overflowX !== 'visible') {
          left = Math.max(left, r.left + p.clientLeft);
          right = Math.min(right, r.left + p.clientLeft + p.clientWidth);
        }
        if (s.overflowY !== 'visible') {
          top = Math.max(top, r.top + p.clientTop);
          bottom = Math.min(bottom, r.top + p.clientTop + p.clientHeight);
        }
      }
      return right > left && bottom > top && rects.some(r =>
        r.bottom > top && r.top < bottom &&
        r.right > left && r.left < right);
    }
    function zimliteSelectLink(index, reveal) {
      for (const e of document.querySelectorAll('[data-zimlite-selected]')) {
        e.style.outline = ''; e.removeAttribute('data-zimlite-selected');
      }
      const e = index >= 0 && index < n ? links[index] : null;
      if (!e) return;
      e.setAttribute('data-zimlite-selected', '');
      e.style.outline = '2px solid #e09020';
      e.focus({preventScroll:true});
      if (reveal) e.scrollIntoView({block:'nearest', inline:'nearest'});
    }
"#;

// Use the same target for input and diagnostics. A link may stay selected far
// above the current viewport after stick scrolling; it must not hijack input.
// Without a visible selected table, choose the wide table nearest the centre.
const TABLE_TARGET_JS: &str = r#"
    function zimliteScrollTable(selected) {
      const visible = t => {
        if (!t || t.scrollWidth <= t.clientWidth + 1) return null;
        const r = t.getBoundingClientRect();
        if (r.width <= 0 || r.height <= 0 || r.bottom <= 0 ||
            r.top >= window.innerHeight || r.right <= 0 || r.left >= window.innerWidth) return null;
        return r;
      };
      const a = document.querySelectorAll('a[href]')[selected];
      const preferred = a && a.closest('.zimlite-table-scroll');
      if (visible(preferred)) return preferred;
      let best = null, distance = Infinity;
      const centre = window.innerHeight / 2;
      for (const t of document.querySelectorAll('.zimlite-table-scroll')) {
        const r = visible(t);
        if (!r) continue;
        const d = Math.max(r.top - centre, centre - r.bottom, 0);
        if (d < distance) { best = t; distance = d; }
      }
      return best;
    }
"#;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Command {
    op: String,
    #[serde(default)]
    selection_id: u64,
    #[serde(default)]
    value: i32,
    #[serde(default)]
    x: i32,
    #[serde(default)]
    y: i32,
    #[serde(default)]
    width: u32,
    #[serde(default)]
    height: u32,
    #[serde(default)]
    scale: f32,
    #[serde(default)]
    text: String,
}

// Fold queued relative motion before rendering, retaining control commands in
// order. Bounded draining prevents input from starving the Servo event loop.
fn next_command(
    commands: &Receiver<Result<Command, String>>,
    deferred: &mut Option<Result<Command, String>>,
) -> Result<Option<(Command, usize)>, String> {
    let mut command = match deferred.take() {
        Some(command) => command?,
        None => match commands.try_recv() {
            Ok(command) => command?,
            Err(mpsc::TryRecvError::Empty) => return Ok(None),
            Err(mpsc::TryRecvError::Disconnected) => return Err("control input closed".into()),
        },
    };
    let mut merged = 0;
    if matches!(command.op.as_str(), "scroll" | "table_scroll_by") {
        for _ in 0..15 {
            match commands.try_recv() {
                Ok(Ok(next)) if next.op == command.op => {
                    command.value = command.value.saturating_add(next.value).clamp(-4096, 4096);
                    merged += 1;
                }
                Ok(next) => {
                    *deferred = Some(next);
                    break;
                }
                Err(_) => break,
            }
        }
    }
    Ok(Some((command, merged)))
}

struct FrameProfile {
    enabled: bool,
    started: Instant,
    previous_frame: Instant,
    frames: u64,
    commands: u64,
    merged: u64,
    paint_us: u128,
    readback_us: u128,
    ipc_us: u128,
    spin_us: u128,
    max_gap_us: u128,
    scroll_commands: u64,
    scroll_delta: i64,
    table_commands: u64,
    table_delta: i64,
}

impl FrameProfile {
    fn new() -> Self {
        Self {
            enabled: std::env::var_os("KIWIX_PROFILE_DIR").is_some(),
            started: Instant::now(),
            previous_frame: Instant::now(),
            frames: 0,
            commands: 0,
            merged: 0,
            paint_us: 0,
            readback_us: 0,
            ipc_us: 0,
            spin_us: 0,
            max_gap_us: 0,
            scroll_commands: 0,
            scroll_delta: 0,
            table_commands: 0,
            table_delta: 0,
        }
    }
    fn report(&mut self) {
        if !self.enabled || self.started.elapsed() < Duration::from_secs(1) {
            return;
        }
        if self.frames > 0 || self.commands > 0 {
            let seconds = self.started.elapsed().as_secs_f64();
            let frames = u128::from(self.frames.max(1));
            eprintln!(
                "servo.profile.frames pid={} fps={:.2} frames={} commands={} merged={} paint_avg_us={} readback_avg_us={} ipc_avg_us={} spin_total_us={} max_gap_ms={} scroll_commands={} scroll_delta={} table_commands={} table_delta={}",
                std::process::id(),
                self.frames as f64 / seconds,
                self.frames,
                self.commands,
                self.merged,
                self.paint_us / frames,
                self.readback_us / frames,
                self.ipc_us / frames,
                self.spin_us,
                self.max_gap_us / 1000,
                self.scroll_commands,
                self.scroll_delta,
                self.table_commands,
                self.table_delta
            );
        }
        let previous = self.previous_frame;
        *self = Self::new();
        self.previous_frame = previous;
    }
}

// Reduce raster work only while moving. Matching HiDPI and framebuffer scale
// retains the CSS viewport, font sizes, and scroll offsets. The client scales
// motion frames on its SDL texture; resizing here would add CPU work and memory.
struct ScrollRaster {
    size: PhysicalSize<u32>,
    motion_scale: f32,
    moving: bool,
    last_motion: Instant,
    transition: Option<Receiver<Result<image::RgbaImage, String>>>,
    transition_started: Instant,
}
impl ScrollRaster {
    fn new(size: PhysicalSize<u32>) -> Self {
        let motion_scale = std::env::var("ZIMLITE_SERVO_SCROLL_SCALE")
            .ok()
            .and_then(|value| value.parse::<f32>().ok())
            .filter(|scale| *scale == 0.5 || *scale == 1.0)
            .unwrap_or(1.0);
        Self {
            size,
            motion_scale,
            moving: false,
            last_motion: Instant::now(),
            transition: None,
            transition_started: Instant::now(),
        }
    }
    fn pending(&self) -> bool {
        self.transition.is_some()
    }
    fn apply(&mut self, webview: &WebView) {
        let scale = if self.moving { self.motion_scale } else { 1.0 };
        webview.resize_with_hidpi_scale_factor(
            PhysicalSize::new(
                (self.size.width as f32 * scale).round().max(1.0) as u32,
                (self.size.height as f32 * scale).round().max(1.0) as u32,
            ),
            euclid::Scale::new(scale),
        );
        // Keep the previous client frame until Servo has painted the final
        // viewport. Resizing invalidates the old compositor scene; ordinary
        // readback here would expose its stale geometry and temporary blanks.
        let (tx, rx) = mpsc::sync_channel(1);
        self.transition = Some(rx);
        self.transition_started = Instant::now();
        webview.take_screenshot(None, move |result| {
            let _ =
                tx.try_send(result.map_err(|error| format!("raster screenshot failed: {error:?}")));
        });
        if std::env::var_os("KIWIX_PROFILE_DIR").is_some() {
            eprintln!(
                "servo.profile.raster scale={scale} width={} height={}",
                self.size.width, self.size.height
            );
        }
    }
    fn completed_frame(&mut self) -> Result<Option<image::RgbaImage>, String> {
        let Some(rx) = self.transition.as_ref() else {
            return Ok(None);
        };
        match rx.try_recv() {
            Ok(result) => {
                self.transition = None;
                let image = result?;
                if std::env::var_os("KIWIX_PROFILE_DIR").is_some() {
                    eprintln!(
                        "servo.profile.raster.ready width={} height={} elapsed_ms={}",
                        image.width(),
                        image.height(),
                        self.transition_started.elapsed().as_millis()
                    );
                }
                Ok(Some(image))
            }
            Err(mpsc::TryRecvError::Empty) => Ok(None),
            Err(mpsc::TryRecvError::Disconnected) => {
                Err("raster screenshot callback closed".into())
            }
        }
    }
    fn motion(&mut self, webview: &WebView) {
        self.last_motion = Instant::now();
        if !self.moving && self.motion_scale < 1.0 {
            self.moving = true;
            self.apply(webview);
        }
    }
    fn idle(&mut self, webview: &WebView) {
        if self.moving && self.last_motion.elapsed() >= Duration::from_secs(1) {
            self.moving = false;
            self.apply(webview);
        }
    }
}

fn read_commands(tx: SyncSender<Result<Command, String>>) {
    let mut input = io::stdin().lock();
    loop {
        let mut length = [0u8; 4];
        if input.read_exact(&mut length).is_err() {
            break;
        }
        let length = u32::from_le_bytes(length) as usize;
        if length == 0 || length > MAX_COMMAND {
            let _ = tx.send(Err("control message exceeds budget".into()));
            break;
        }
        let mut data = vec![0; length];
        if input.read_exact(&mut data).is_err() {
            break;
        }
        let command = serde_json::from_slice(&data).map_err(|e| e.to_string());
        if tx.send(command).is_err() {
            break;
        }
    }
}

pub fn write_frame(image: &image::RgbaImage) -> io::Result<()> {
    let mut out = io::stdout().lock();
    out.write_all(b"ZSR1")?;
    out.write_all(&image.width().to_le_bytes())?;
    out.write_all(&image.height().to_le_bytes())?;
    out.write_all(image.as_raw())?;
    out.flush()
}
pub(crate) fn write_event(event: Value) -> Result<(), Box<dyn Error>> {
    let data = serde_json::to_vec(&event)?;
    if data.len() > MAX_EVENT {
        return Err("event exceeds protocol budget".into());
    }
    let mut out = io::stdout().lock();
    out.write_all(b"ZSE1")?;
    out.write_all(&(data.len() as u32).to_le_bytes())?;
    out.write_all(&data)?;
    out.flush()?;
    Ok(())
}

fn evaluate_state(
    webview: &WebView,
    selection_id: u64,
    tx: SyncSender<Value>,
    pending: Rc<Cell<bool>>,
) {
    if pending.replace(true) {
        return;
    }
    // Trusted viewer glue; article scripts remain blocked by CSP.
    // Restrict the selectable link list and all strings returned through IPC.
    webview.evaluate_javascript(
        format!(
            r#"(() => {{
      {LINK_STATE_JS}
      const i = selected >= 0 && selected < n ? selected : -1;
      {TABLE_TARGET_JS}
      const table = zimliteScrollTable(i);
      const tableScrollLeft = !!table && table.scrollLeft > 1;
      const tableScrollRight = !!table && table.scrollLeft + table.clientWidth < table.scrollWidth - 1;
      const body = document.body;
      let contentReady = false;
      if (location.protocol === 'http:' && body && body.getBoundingClientRect().height > 0) {{
        // Inspect a bounded prefix of text nodes, not the full article text.
        const walker = document.createTreeWalker(body, NodeFilter.SHOW_TEXT);
        for (let checked = 0, node; checked < 64 && (node = walker.nextNode()); checked++) {{
          const tag = node.parentElement && node.parentElement.tagName;
          if (tag === 'STYLE' || tag === 'SCRIPT' || tag === 'NOSCRIPT') continue;
          if (/\S/.test(node.nodeValue.slice(0,256))) {{contentReady = true; break;}}
        }}
        if (!contentReady) contentReady = !!body.querySelector('img,svg,canvas,math');
      }}
      return JSON.stringify({{event:'state', title:document.title.slice(0,256),
        content_ready:contentReady,
        dom_ready:location.protocol==='http:' && document.readyState!=='loading',
        scroll_y:Math.round(window.scrollY),
        scroll_height:Math.round((document.scrollingElement||document.documentElement).scrollHeight),
        viewport_height:Math.round(window.innerHeight),
        table_scroll_x:table?Math.round(table.scrollLeft):0,
        table_scroll_width:table?Math.round(table.scrollWidth):0,
        table_client_width:table?Math.round(table.clientWidth):0,
        link_count:n, selected_link:i, selection_id:{selection_id},
        selected_url:i < 0 ? '' : links[i].href.slice(0,8192),
        table_scroll_left:tableScrollLeft, table_scroll_right:tableScrollRight}});
    }})()"#
        ),
        move |result| {
            pending.set(false);
            if let Ok(JSValue::String(value)) = result {
                if value.len() <= MAX_EVENT {
                    if let Ok(event) = serde_json::from_str(&value) {
                        let _ = tx.try_send(event);
                    }
                }
            }
        },
    );
}

pub fn run(
    servo: &Servo,
    webview: &WebView,
    context: Rc<dyn RenderingContext>,
    delegate: &crate::Delegate,
    wake: &Receiver<()>,
    events_tx: SyncSender<Value>,
    events_rx: Receiver<Value>,
) -> Result<(), Box<dyn Error>> {
    let (tx, commands) = mpsc::sync_channel(16);
    std::thread::spawn(move || read_commands(tx));
    let mut selection_id = 0;
    let mut last_frame = Instant::now();
    let command_pending = Rc::new(Cell::new(false));
    let state_pending = Rc::new(Cell::new(false));
    let mut command_started = Instant::now();
    let mut deferred_command = None;
    let mut last_state = Instant::now();
    let mut profile = FrameProfile::new();
    let mut raster = ScrollRaster::new(context.size());
    let mut last_load_status = servo::LoadStatus::Started;
    let mut content_ready = false;
    let mut dom_ready = false;
    let load_started = Instant::now();
    let mut published_frame = false;
    // Restore commands may arrive before their anchors/links exist. Retain
    // only the latest absolute position, selection and anchor (at most three)
    // until the whole HTML has been parsed, including targets near its end.
    let mut initial_commands: Vec<Command> = Vec::new();
    evaluate_state(
        webview,
        selection_id,
        events_tx.clone(),
        state_pending.clone(),
    );
    loop {
        if !published_frame && load_started.elapsed() > crate::TIMEOUT {
            return Err("timed out waiting for the first document frame".into());
        }
        let load_status = delegate.load_status.get();
        if load_status != last_load_status {
            let phase = match load_status {
                servo::LoadStatus::Started => "document",
                servo::LoadStatus::HeadParsed => "resources",
                servo::LoadStatus::Complete => "complete",
            };
            write_event(json!({"event":"progress", "phase":phase}))?;
            last_load_status = load_status;
            if load_status == servo::LoadStatus::Complete {
                delegate.frame.set(true);
            }
        }
        if command_pending.get() && command_started.elapsed() > Duration::from_secs(5) {
            return Err("viewer command timed out".into());
        }
        if raster.pending() && raster.transition_started.elapsed() > Duration::from_secs(5) {
            return Err("raster transition timed out".into());
        }
        if !command_pending.get() && !raster.pending() {
            let ready = content_ready || load_status == servo::LoadStatus::Complete;
            let document_ready = dom_ready || load_status == servo::LoadStatus::Complete;
            let next = if document_ready && !initial_commands.is_empty() {
                Some((initial_commands.remove(0), 0))
            } else {
                next_command(&commands, &mut deferred_command)?
            };
            match next {
                None => {}
                Some((command, merged)) => {
                    if !document_ready {
                        match command.op.as_str() {
                            "scroll_to" | "select" | "anchor" => {
                                if let Some(index) =
                                    initial_commands.iter().position(|c| c.op == command.op)
                                {
                                    initial_commands[index] = command;
                                } else {
                                    initial_commands.push(command);
                                }
                                continue;
                            }
                            _ => {}
                        }
                    }
                    if !ready
                        && matches!(
                            command.op.as_str(),
                            "scroll" | "table_scroll" | "table_scroll_by" | "move_link" | "click"
                        )
                    {
                        continue;
                    }
                    profile.merged += merged as u64;
                    if matches!(
                        command.op.as_str(),
                        "scroll" | "table_scroll" | "table_scroll_by"
                    ) {
                        raster.motion(webview);
                        if raster.pending() {
                            // Apply input only after the resized hit-test and
                            // scroll trees are ready, preserving motion order.
                            deferred_command = Some(Ok(command));
                            continue;
                        }
                    }
                    profile.commands += 1;
                    if command.op == "scroll" {
                        profile.scroll_commands += 1;
                        profile.scroll_delta += i64::from(command.value.clamp(-4096, 4096));
                    } else if command.op == "table_scroll_by" {
                        profile.table_commands += 1;
                        profile.table_delta += i64::from(command.value.clamp(-4096, 4096));
                    }
                    command_started = Instant::now();
                    if matches!(command.op.as_str(), "select" | "move_link") {
                        selection_id = command.selection_id;
                    }
                    let script = match command.op.as_str() {
                        "stop" => return Ok(()),
                        "scroll" => {
                            // Compositor scrolling avoids a JS round trip and
                            // synchronous DOM scrolling/reflow for every tick.
                            // A point in the outer margin targets the root,
                            // preserving horizontal table isolation.
                            webview.notify_scroll_event(
                                Scroll::Delta(WebViewVector::Page(euclid::Vector2D::new(
                                    0.0,
                                    command.value.clamp(-4096, 4096) as f32,
                                ))),
                                WebViewPoint::Device(DevicePoint::new(1.0, 1.0)),
                            );
                            None
                        }
                        "scroll_to" => {
                            Some(format!("window.scrollTo(0,{});", command.value.max(0)))
                        }
                        "table_scroll" | "table_scroll_by" => Some(format!(
                            r#"(() => {{
                              {TABLE_TARGET_JS}
                              {LINK_STATE_JS}
                              const t=zimliteScrollTable(selected);
                              if(t){{const max=t.scrollWidth-t.clientWidth;
                                t.scrollLeft=Math.max(0,Math.min(max,t.scrollLeft+({offset})));}}
                            }})()"#,
                            offset = if command.op == "table_scroll_by" {
                                command.value.clamp(-4096, 4096).to_string()
                            } else {
                                format!("{}*Math.max(80,t.clientWidth*.75)", command.value.signum())
                            },
                        )),
                        "select" => Some(format!(
                            r#"(() => {{
                              {LINK_STATE_JS}
                              {LINK_SELECTION_JS}
                              zimliteSelectLink({}, true);
                            }})()"#,
                            command.value.clamp(-1, 2047),
                        )),
                        "move_link" => Some(format!(
                            r#"(() => {{
                              {LINK_STATE_JS}
                              {LINK_SELECTION_JS}
                              if (!n) return;
                              if (selected < 0 || !zimliteLinkVisible(links[selected])) {{
                                for (let i = 0; i < n; i++) {{
                                  if (zimliteLinkVisible(links[i])) {{
                                    zimliteSelectLink(i, false); return;
                                  }}
                                }}
                                return;
                              }}
                              for (let step = 1; step <= n; step++) {{
                                const i = (selected + ({direction}) * step + n) % n;
                                if (links[i].getClientRects().length &&
                                    getComputedStyle(links[i]).visibility === 'visible') {{
                                  zimliteSelectLink(i, !zimliteLinkVisible(links[i])); return;
                                }}
                              }}
                            }})()"#,
                            direction = if command.value < 0 { -1 } else { 1 },
                        )),
                        "anchor" => {
                            let id = serde_json::to_string(&command.text)?;
                            Some(format!(
                                "(() => {{const e=document.getElementById({id})||document.getElementsByName({id})[0]; if(e)e.scrollIntoView();}})()"
                            ))
                        }
                        "click" => {
                            let (x, y) = (command.x, command.y);
                            let tx = events_tx.clone();
                            command_pending.set(true);
                            let pending = command_pending.clone();
                            webview.evaluate_javascript(format!("(() => {{const e=document.elementFromPoint({x},{y}); const a=e&&e.closest('a[href]');return a?a.href.slice(0,8192):'';}})()"), move |result| {
                            pending.set(false);
                            if let Ok(JSValue::String(url))=result { if !url.is_empty(){ let _=tx.try_send(json!({"event":"navigate","url":url})); } }
                        });
                            None
                        }
                        "resize" => {
                            let size = PhysicalSize::new(command.width, command.height);
                            if size.width == 0
                                || size.height == 0
                                || size.width > 1280
                                || size.height > 1280
                                || u64::from(size.width) * u64::from(size.height)
                                    > u64::from(crate::MAX_PIXELS)
                            {
                                return Err("resize exceeds pixel budget".into());
                            }
                            raster.size = size;
                            raster.apply(webview);
                            None
                        }
                        "zoom" => {
                            if !command.scale.is_finite() || !(0.5..=2.0).contains(&command.scale) {
                                return Err("zoom outside budget".into());
                            }
                            webview.set_page_zoom(command.scale);
                            None
                        }
                        "theme" => {
                            let dark = command.text == "dark";
                            if !dark && command.text != "light" {
                                return Err("unknown theme".into());
                            }
                            webview.notify_theme_change(if dark {
                                servo::Theme::Dark
                            } else {
                                servo::Theme::Light
                            });
                            None
                        }
                        "state" => None,
                        _ => return Err("unknown control operation".into()),
                    };
                    if let Some(script) = script {
                        command_pending.set(true);
                        let pending = command_pending.clone();
                        webview.evaluate_javascript(script, move |_| {
                            pending.set(false);
                        });
                    }
                    if command.op != "scroll" {
                        evaluate_state(
                            webview,
                            selection_id,
                            events_tx.clone(),
                            state_pending.clone(),
                        );
                        last_state = Instant::now();
                    }
                }
            }
        }
        if !command_pending.get() && !raster.pending() {
            raster.idle(webview);
        }
        let spin_started = Instant::now();
        servo.spin_event_loop();
        profile.spin_us += spin_started.elapsed().as_micros();
        for event in events_rx.try_iter() {
            if event.get("event").and_then(Value::as_str) == Some("state") {
                let ready = event
                    .get("content_ready")
                    .and_then(Value::as_bool)
                    .unwrap_or(false);
                if ready && !content_ready {
                    delegate.frame.set(true);
                }
                content_ready = ready;
                dom_ready = event
                    .get("dom_ready")
                    .and_then(Value::as_bool)
                    .unwrap_or(false);
            }
            write_event(event)?;
        }
        if (delegate.frame.get() || raster.pending())
            && last_frame.elapsed() >= Duration::from_millis(33)
        {
            let paint_started = Instant::now();
            crate::gl_profile::begin_frame();
            if delegate.frame.replace(false) {
                // The parent watchdog can abort a blocked driver call even
                // when the helper cannot return to its event loop.
                write_event(json!({"event":"paint", "phase":"begin"}))?;
                webview.paint();
                write_event(json!({"event":"paint", "phase":"end"}))?;
            }
            let paint_elapsed = paint_started.elapsed();
            crate::gl_profile::end_frame(paint_elapsed);
            profile.paint_us += paint_elapsed.as_micros();
            let size = context.size();
            let rect =
                DeviceIntRect::from_size(DeviceIntSize::new(size.width as i32, size.height as i32));
            let read_started = Instant::now();
            let image = if raster.pending() {
                raster.completed_frame()?
            } else {
                context.read_to_image(rect)
            };
            profile.readback_us += read_started.elapsed().as_micros();
            // A document's first visible layout can precede load completion by
            // seconds. Publish it without waiting for offscreen images/fonts.
            // Keep about:blank/startup frames out of the first-frame watchdog.
            if let Some(image) = image.filter(|_| {
                content_ready || delegate.load_status.get() == servo::LoadStatus::Complete
            }) {
                let ipc_started = Instant::now();
                write_frame(&image)?;
                if !published_frame {
                    published_frame = true;
                }
                profile.ipc_us += ipc_started.elapsed().as_micros();
                profile.frames += 1;
                let now = Instant::now();
                profile.max_gap_us = profile
                    .max_gap_us
                    .max(now.duration_since(profile.previous_frame).as_micros());
                profile.previous_frame = now;
            }
            last_frame = Instant::now();
        }
        if !raster.pending() && last_state.elapsed() >= Duration::from_millis(200) {
            evaluate_state(
                webview,
                selection_id,
                events_tx.clone(),
                state_pending.clone(),
            );
            last_state = Instant::now();
        }
        profile.report();
        let _ = wake.recv_timeout(Duration::from_millis(16));
    }
}
