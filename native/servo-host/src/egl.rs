//! Headless GLES pbuffer. EGL/GLES are loaded from the firmware at runtime.
//! All handles belong to the Servo event thread; no display window is created.
use std::cell::Cell;
use std::ffi::{CString, c_char, c_void};
use std::ptr;
use std::rc::Rc;
use std::sync::Arc;

use dpi::PhysicalSize;
use gleam::gl::{self, Gl};
use image::RgbaImage;
use libloading::Library;
use servo::{DeviceIntRect, RenderingContext};

type Handle = *mut c_void;
type Boolean = u32;
const NONE: i32 = 0x3038;

struct Api {
    _egl: Library,
    _gles: Library,
    initialize: unsafe extern "C" fn(Handle, *mut i32, *mut i32) -> Boolean,
    terminate: unsafe extern "C" fn(Handle) -> Boolean,
    get_display: unsafe extern "C" fn(Handle) -> Handle,
    bind_api: unsafe extern "C" fn(u32) -> Boolean,
    choose_config: unsafe extern "C" fn(Handle, *const i32, *mut Handle, i32, *mut i32) -> Boolean,
    create_surface: unsafe extern "C" fn(Handle, Handle, *const i32) -> Handle,
    destroy_surface: unsafe extern "C" fn(Handle, Handle) -> Boolean,
    create_context: unsafe extern "C" fn(Handle, Handle, Handle, *const i32) -> Handle,
    destroy_context: unsafe extern "C" fn(Handle, Handle) -> Boolean,
    make_current: unsafe extern "C" fn(Handle, Handle, Handle, Handle) -> Boolean,
    get_error: unsafe extern "C" fn() -> u32,
    get_proc: unsafe extern "C" fn(*const c_char) -> *const c_void,
}
impl Api {
    fn load() -> Result<Self, Box<dyn std::error::Error>> {
        // EGL function signatures and tokens follow Khronos EGL 1.4/1.5.
        unsafe {
            let egl = Library::new("libEGL.so.1")?;
            let gles = Library::new("libGLESv2.so.2")?;
            Ok(Self {
                initialize: *egl.get(b"eglInitialize\0")?,
                terminate: *egl.get(b"eglTerminate\0")?,
                get_display: *egl.get(b"eglGetDisplay\0")?,
                bind_api: *egl.get(b"eglBindAPI\0")?,
                choose_config: *egl.get(b"eglChooseConfig\0")?,
                create_surface: *egl.get(b"eglCreatePbufferSurface\0")?,
                destroy_surface: *egl.get(b"eglDestroySurface\0")?,
                create_context: *egl.get(b"eglCreateContext\0")?,
                destroy_context: *egl.get(b"eglDestroyContext\0")?,
                make_current: *egl.get(b"eglMakeCurrent\0")?,
                get_error: *egl.get(b"eglGetError\0")?,
                get_proc: *egl.get(b"eglGetProcAddress\0")?,
                _egl: egl,
                _gles: gles,
            })
        }
    }
    fn proc(&self, name: &str) -> *const c_void {
        let symbol = CString::new(name).expect("GL function name");
        let address = unsafe {
            // PowerVR does not expose core GLES symbols via eglGetProcAddress.
            self._gles
                .get::<*const c_void>(symbol.as_bytes_with_nul())
                .map(|p| *p)
                .unwrap_or_else(|_| (self.get_proc)(symbol.as_ptr()))
        };
        crate::gl_profile::wrap(name, address)
    }
    fn error(&self, operation: &str) -> Box<dyn std::error::Error> {
        format!("{operation} failed: EGL 0x{:x}", unsafe {
            (self.get_error)()
        })
        .into()
    }
}
struct Handles {
    api: Api,
    display: Handle,
    config: Handle,
    context: Handle,
    surface: Cell<Handle>,
}
impl Drop for Handles {
    fn drop(&mut self) {
        unsafe {
            if !self.display.is_null() {
                (self.api.make_current)(
                    self.display,
                    ptr::null_mut(),
                    ptr::null_mut(),
                    ptr::null_mut(),
                );
                if !self.surface.get().is_null() {
                    (self.api.destroy_surface)(self.display, self.surface.get());
                }
                if !self.context.is_null() {
                    (self.api.destroy_context)(self.display, self.context);
                }
                (self.api.terminate)(self.display);
            }
        }
    }
}
impl Handles {
    fn surface(&self, size: PhysicalSize<u32>) -> Handle {
        let attrs = [0x3057, size.width as i32, 0x3056, size.height as i32, NONE];
        unsafe { (self.api.create_surface)(self.display, self.config, attrs.as_ptr()) }
    }
}

pub struct EglContext {
    handles: Handles,
    size: Cell<PhysicalSize<u32>>,
    gleam: Rc<dyn Gl>,
    glow: Arc<glow::Context>,
}
impl EglContext {
    pub fn new(size: PhysicalSize<u32>) -> Result<Self, Box<dyn std::error::Error>> {
        let api = Api::load()?;
        let mut display = unsafe { (api.get_display)(ptr::null_mut()) };
        let (mut major, mut minor) = (0, 0);
        unsafe {
            if (api.initialize)(display, &mut major, &mut minor) == 0 {
                // Mesa can render without X11/Wayland through its surfaceless
                // platform. Firmware drivers keep their default-display path.
                let platform = (api.get_proc)(c"eglGetPlatformDisplayEXT".as_ptr());
                if platform.is_null() {
                    return Err(api.error("eglInitialize"));
                }
                let get_platform: unsafe extern "C" fn(u32, Handle, *const i32) -> Handle =
                    std::mem::transmute(platform);
                display = get_platform(0x31DD, ptr::null_mut(), ptr::null());
                if (api.initialize)(display, &mut major, &mut minor) == 0 {
                    return Err(api.error("surfaceless eglInitialize"));
                }
            }
        }
        let mut handles = Handles {
            api,
            display,
            config: ptr::null_mut(),
            context: ptr::null_mut(),
            surface: Cell::new(ptr::null_mut()),
        };
        unsafe {
            if (handles.api.bind_api)(0x30A0) == 0 {
                return Err(handles.api.error("eglBindAPI"));
            }
            let attrs = [
                0x3033, 1, 0x3040, 0x0040, 0x3024, 8, 0x3023, 8, 0x3022, 8, 0x3021, 8, 0x3025, 24,
                0x3026, 8, NONE,
            ];
            let mut count = 0;
            if (handles.api.choose_config)(
                display,
                attrs.as_ptr(),
                &mut handles.config,
                1,
                &mut count,
            ) == 0
                || count == 0
            {
                return Err(handles.api.error("eglChooseConfig GLES3 pbuffer"));
            }
            handles.surface.set(handles.surface(size));
            if handles.surface.get().is_null() {
                return Err(handles.api.error("eglCreatePbufferSurface"));
            }
            let attrs = [0x3098, 3, NONE];
            handles.context = (handles.api.create_context)(
                display,
                handles.config,
                ptr::null_mut(),
                attrs.as_ptr(),
            );
            if handles.context.is_null()
                || (handles.api.make_current)(
                    display,
                    handles.surface.get(),
                    handles.surface.get(),
                    handles.context,
                ) == 0
            {
                return Err(handles.api.error("eglCreateContext / eglMakeCurrent"));
            }
        }
        let gleam = unsafe { gl::GlesFns::load_with(|name| handles.api.proc(name)) };
        let glow = unsafe { glow::Context::from_loader_function(|name| handles.api.proc(name)) };
        eprintln!(
            "EGL {major}.{minor}; {} / {}",
            gleam.get_string(gl::VERSION),
            gleam.get_string(gl::RENDERER)
        );
        Ok(Self {
            handles,
            size: Cell::new(size),
            gleam,
            glow: Arc::new(glow),
        })
    }
}
impl RenderingContext for EglContext {
    fn prepare_for_rendering(&self) {
        self.gleam.bind_framebuffer(gl::FRAMEBUFFER, 0);
    }
    fn read_to_image(&self, rect: DeviceIntRect) -> Option<RgbaImage> {
        let size = self.size.get();
        let (x, y, w, h) = (
            rect.min.x,
            rect.min.y,
            rect.max.x.checked_sub(rect.min.x)?,
            rect.max.y.checked_sub(rect.min.y)?,
        );
        if x < 0
            || y < 0
            || w <= 0
            || h <= 0
            || x.checked_add(w)? > size.width as i32
            || y.checked_add(h)? > size.height as i32
        {
            return None;
        }
        self.prepare_for_rendering();
        let pixels = self.gleam.read_pixels(
            x,
            size.height as i32 - y - h,
            w,
            h,
            gl::RGBA,
            gl::UNSIGNED_BYTE,
        );
        let mut image = RgbaImage::from_raw(w as u32, h as u32, pixels)?;
        image::imageops::flip_vertical_in_place(&mut image);
        Some(image)
    }
    fn size(&self) -> PhysicalSize<u32> {
        self.size.get()
    }
    fn resize(&self, size: PhysicalSize<u32>) {
        assert!(
            size.width > 0
                && size.height > 0
                && u64::from(size.width) * u64::from(size.height) <= 1280 * 720
        );
        if size == self.size.get() {
            return;
        }
        let h = &self.handles;
        let next = h.surface(size);
        assert!(!next.is_null(), "EGL pbuffer resize failed");
        unsafe {
            if (h.api.make_current)(h.display, next, next, h.context) == 0 {
                (h.api.destroy_surface)(h.display, next);
                panic!("EGL resize make current failed");
            }
            (h.api.destroy_surface)(h.display, h.surface.replace(next));
        }
        self.size.set(size);
    }
    // Keep the pbuffer back buffer intact until the requested readback.
    fn present(&self) {
        self.gleam.flush();
    }
    fn make_current(&self) -> Result<(), surfman::Error> {
        let h = &self.handles;
        if unsafe { (h.api.make_current)(h.display, h.surface.get(), h.surface.get(), h.context) }
            == 0
        {
            Err(surfman::Error::Failed)
        } else {
            Ok(())
        }
    }
    fn gleam_gl_api(&self) -> Rc<dyn Gl> {
        self.gleam.clone()
    }
    fn glow_gl_api(&self) -> Arc<glow::Context> {
        self.glow.clone()
    }
}
