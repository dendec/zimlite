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
use servo::{DeviceIntRect, DeviceIntSize, JSValue, RenderingContext, Servo, WebView};

const MAX_COMMAND: usize = 8192;
const MAX_EVENT: usize = 65536;
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Command {
    op: String,
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
fn write_event(event: Value) -> Result<(), Box<dyn Error>> {
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
    selected: i32,
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
      const links = document.querySelectorAll('a[href]');
      const n = Math.min(links.length, 2048);
      const i = n === 0 ? -1 : Math.max(-1, Math.min({selected}, n - 1));
      const selectedElement = i < 0 ? null : links[i];
      const table = selectedElement ? selectedElement.closest('.zimlite-table-scroll') : null;
      const tableScrollLeft = !!table && table.scrollLeft > 1;
      const tableScrollRight = !!table && table.scrollLeft + table.clientWidth < table.scrollWidth - 1;
      return JSON.stringify({{event:'state', title:document.title.slice(0,256),
        scroll_y:Math.round(window.scrollY), link_count:n, selected_link:i,
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
    let mut selected = -1;
    let mut last_frame = Instant::now();
    let command_pending = Rc::new(Cell::new(false));
    let state_pending = Rc::new(Cell::new(false));
    let mut command_started = Instant::now();
    evaluate_state(webview, selected, events_tx.clone(), state_pending.clone());
    loop {
        if command_pending.get() && command_started.elapsed() > Duration::from_secs(5) {
            return Err("viewer command timed out".into());
        }
        if !command_pending.get() {
            match commands.try_recv() {
                Ok(Err(error)) => return Err(error.into()),
                Err(mpsc::TryRecvError::Disconnected) => return Ok(()),
                Err(mpsc::TryRecvError::Empty) => {}
                Ok(Ok(command)) => {
                    command_started = Instant::now();
                    let script = match command.op.as_str() {
                        "stop" => return Ok(()),
                        "scroll" => Some(format!(
                            "window.scrollBy(0,{});",
                            command.value.clamp(-4096, 4096)
                        )),
                        "scroll_to" => {
                            Some(format!("window.scrollTo(0,{});", command.value.max(0)))
                        }
                        "table_scroll" => Some(format!(
                            r#"(() => {{
                              const a=document.querySelectorAll('a[href]')[{selected}];
                              const t=a&&a.closest('.zimlite-table-scroll');
                              if(t){{const max=t.scrollWidth-t.clientWidth;
                                t.scrollLeft=Math.max(0,Math.min(max,t.scrollLeft+({direction})*Math.max(80,t.clientWidth*.75)));}}
                            }})()"#,
                            direction = command.value.signum(),
                        )),
                        "select" => {
                            selected = command.value.clamp(-1, 2047);
                            Some(format!(
                                r#"(() => {{ const a=document.querySelectorAll('a[href]');
                          for(const e of document.querySelectorAll('[data-zimlite-selected]')) {{
                            e.style.outline=''; e.removeAttribute('data-zimlite-selected'); }}
                          const e=a[{selected}]; if(e){{e.setAttribute('data-zimlite-selected','');
                            e.style.outline='2px solid #e09020'; e.focus(); e.scrollIntoView({{block:'nearest'}});}}
                        }})()"#
                            ))
                        }
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
                            webview.resize(size);
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
                    evaluate_state(webview, selected, events_tx.clone(), state_pending.clone());
                }
            }
        }
        servo.spin_event_loop();
        for event in events_rx.try_iter() {
            write_event(event)?;
        }
        if delegate.frame.get() && last_frame.elapsed() >= Duration::from_millis(33) {
            delegate.frame.set(false);
            webview.paint();
            let size = context.size();
            let rect =
                DeviceIntRect::from_size(DeviceIntSize::new(size.width as i32, size.height as i32));
            if let Some(image) = context.read_to_image(rect) {
                write_frame(&image)?;
            }
            evaluate_state(webview, selected, events_tx.clone(), state_pending.clone());
            last_frame = Instant::now();
        }
        let _ = wake.recv_timeout(Duration::from_millis(16));
    }
}
