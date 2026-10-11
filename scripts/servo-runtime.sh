#!/bin/sh
# This launcher must exec: the Go watchdog follows this exact PID.
set -eu
runtime_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
case $(uname -m) in
    aarch64|arm64) runtime_arch=aarch64 ;;
    x86_64|amd64) runtime_arch=amd64 ;;
    *) echo 'Servo runtime is unavailable for this architecture' >&2; exit 1 ;;
esac
runtime_prefix="$runtime_dir/$runtime_arch"
[ -x "$runtime_prefix/bin/zimlite-servo-host" ] || {
    echo 'Servo runtime is not installed' >&2; exit 1;
}
export LD_LIBRARY_PATH="$runtime_prefix/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
# Fontconfig 2.13.1 does not resolve prefix="relative" consistently on the
# handheld firmware. Generate an absolute directory without depending on CWD.
# Keep mutable caches off FUSE/exFAT. Fontconfig and Mesa mmap/write their
# caches while article resources and executable pages are being read.
font_config_dir="${TMPDIR:-/tmp}/zimlite-servo-cache-$(id -u)"
mkdir -p "$font_config_dir"
export XDG_CACHE_HOME="$font_config_dir"
export MESA_SHADER_CACHE_DIR="$font_config_dir/mesa"
export MESA_GLSL_CACHE_DIR="$font_config_dir/mesa"
export MESA_SHADER_CACHE_MAX_SIZE=8M MESA_GLSL_CACHE_MAX_SIZE=8M
font_config_tmp=$(mktemp "$font_config_dir/fonts.conf.XXXXXX")
trap 'rm -f "$font_config_tmp"' EXIT HUP INT TERM
font_directory_xml=$(printf '%s' "$runtime_dir/fonts" | sed 's/\&/\&amp;/g; s/</\&lt;/g; s/>/\&gt;/g')
font_directory_sed=$(printf '%s' "$font_directory_xml" | sed 's/[\\&|]/\\&/g')
sed "s|<dir prefix=\"relative\">fonts</dir>|<dir>$font_directory_sed</dir>|" \
    "$runtime_dir/fonts.conf" > "$font_config_tmp"
mv -f "$font_config_tmp" "$font_config_dir/fonts.conf"
trap - EXIT HUP INT TERM
export FONTCONFIG_FILE="$font_config_dir/fonts.conf"
export FONTCONFIG_PATH="$runtime_dir"
export ZIMLITE_SERVO_SOFTWARE_LIB_DIR="$runtime_prefix/software"
# Concurrent SDL/Servo firmware contexts can freeze the whole handheld before
# the process watchdog can intervene. Keep hardware rendering an explicit opt-in
# until that driver/device combination has been validated with the full app.
export ZIMLITE_SERVO_CONTEXT="${ZIMLITE_SERVO_CONTEXT:-software}"
if [ "$ZIMLITE_SERVO_CONTEXT" = software ]; then
    export LD_LIBRARY_PATH="$runtime_prefix/software:$LD_LIBRARY_PATH"
    export LIBGL_DRIVERS_PATH="$runtime_prefix/software/dri"
    export __EGL_VENDOR_LIBRARY_FILENAMES="$runtime_prefix/software/egl_vendor.json"
    export LIBGL_ALWAYS_SOFTWARE=1 GALLIUM_DRIVER=llvmpipe LP_NUM_THREADS=2 EGL_PLATFORM=surfaceless
    export ZIMLITE_SERVO_CONTEXT=gles
fi
exec "$runtime_prefix/bin/zimlite-servo-host" "$@"
