//! Optional GL timings at the driver boundary, without changing rendering.
use std::cell::RefCell;
use std::ffi::c_void;
use std::sync::OnceLock;
use std::time::{Duration, Instant};

#[derive(Default)]
struct Timings {
    calls: u64,
    driver_us: u128,
    longest_us: u128,
    longest_call: &'static str,
}

thread_local! {
    static TIMINGS: RefCell<Timings> = RefCell::default();
}

fn enabled() -> bool {
    static ENABLED: OnceLock<bool> = OnceLock::new();
    *ENABLED.get_or_init(|| std::env::var("ZIMLITE_SERVO_GL_PROFILE").as_deref() == Ok("1"))
}

fn record(name: &'static str, elapsed: Duration) {
    let us = elapsed.as_micros();
    TIMINGS.with_borrow_mut(|timings| {
        timings.calls += 1;
        timings.driver_us += us;
        if us > timings.longest_us {
            timings.longest_us = us;
            timings.longest_call = name;
        }
    });
    if elapsed >= Duration::from_millis(100) {
        eprintln!(
            "servo.profile.gl pid={} call={name} elapsed_us={us}",
            std::process::id()
        );
    }
}

pub fn begin_frame() {
    if enabled() {
        TIMINGS.with_borrow_mut(|timings| *timings = Timings::default());
    }
}

pub fn end_frame(elapsed: Duration) {
    if enabled() && elapsed >= Duration::from_millis(100) {
        TIMINGS.with_borrow(|timings| {
            eprintln!(
                "servo.profile.paint pid={} elapsed_us={} gl_us={} gl_calls={} longest_call={} longest_us={}",
                std::process::id(), elapsed.as_micros(), timings.driver_us,
                timings.calls, timings.longest_call, timings.longest_us
            );
        });
    }
}

// Each symbol has the exact GLES ABI. The native addresses remain valid for
// the lifetime of EglContext, which owns the loaded driver libraries.
macro_rules! timed {
    ($slot:ident, $wrapper:ident, ($($arg:ident: $ty:ty),*)) => {
        static $slot: OnceLock<usize> = OnceLock::new();
        unsafe extern "system" fn $wrapper($($arg: $ty),*) {
            let original: unsafe extern "system" fn($($ty),*) =
                unsafe { std::mem::transmute(*$slot.get().expect("loaded GL symbol")) };
            let started = Instant::now();
            unsafe { original($($arg),*) };
            record(stringify!($wrapper), started.elapsed());
        }
    };
}

timed!(COMPILE_SHADER, compile_shader, (shader: u32));
timed!(LINK_PROGRAM, link_program, (program: u32));
timed!(DRAW_ARRAYS, draw_arrays, (mode: u32, first: i32, count: i32));
timed!(DRAW_ELEMENTS, draw_elements, (mode: u32, count: i32, kind: u32, indices: *const c_void));
timed!(DRAW_ARRAYS_INSTANCED, draw_arrays_instanced, (mode: u32, first: i32, count: i32, instances: i32));
timed!(DRAW_ELEMENTS_INSTANCED, draw_elements_instanced, (mode: u32, count: i32, kind: u32, indices: *const c_void, instances: i32));
timed!(BUFFER_DATA, buffer_data, (target: u32, size: isize, data: *const c_void, usage: u32));
timed!(TEX_IMAGE_2D, tex_image_2d, (target: u32, level: i32, internal_format: i32, width: i32, height: i32, border: i32, format: u32, kind: u32, pixels: *const c_void));
timed!(TEX_SUB_IMAGE_2D, tex_sub_image_2d, (target: u32, level: i32, x: i32, y: i32, width: i32, height: i32, format: u32, kind: u32, pixels: *const c_void));
timed!(FINISH, finish, ());

pub fn wrap(name: &str, address: *const c_void) -> *const c_void {
    if !enabled() || address.is_null() {
        return address;
    }
    macro_rules! symbol {
        ($slot:ident, $wrapper:ident) => {{
            // Refuse to reuse a wrapper if another driver is ever loaded.
            let stored = $slot.get_or_init(|| address as usize);
            if *stored != address as usize {
                return address;
            }
            $wrapper as *const c_void
        }};
    }
    match name {
        "glCompileShader" => symbol!(COMPILE_SHADER, compile_shader),
        "glLinkProgram" => symbol!(LINK_PROGRAM, link_program),
        "glDrawArrays" => symbol!(DRAW_ARRAYS, draw_arrays),
        "glDrawElements" => symbol!(DRAW_ELEMENTS, draw_elements),
        "glDrawArraysInstanced" => symbol!(DRAW_ARRAYS_INSTANCED, draw_arrays_instanced),
        "glDrawElementsInstanced" => symbol!(DRAW_ELEMENTS_INSTANCED, draw_elements_instanced),
        "glBufferData" => symbol!(BUFFER_DATA, buffer_data),
        "glTexImage2D" => symbol!(TEX_IMAGE_2D, tex_image_2d),
        "glTexSubImage2D" => symbol!(TEX_SUB_IMAGE_2D, tex_sub_image_2d),
        "glFinish" => symbol!(FINISH, finish),
        _ => address,
    }
}
