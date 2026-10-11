#!/bin/bash

XDG_DATA_HOME=${XDG_DATA_HOME:-$HOME/.local/share}

if [ -d "/opt/system/Tools/PortMaster/" ]; then
  controlfolder="/opt/system/Tools/PortMaster"
elif [ -d "/opt/tools/PortMaster/" ]; then
  controlfolder="/opt/tools/PortMaster"
elif [ -d "$XDG_DATA_HOME/PortMaster/" ]; then
  controlfolder="$XDG_DATA_HOME/PortMaster"
else
  controlfolder="/roms/ports/PortMaster"
fi

source $controlfolder/control.txt
[ -f "${controlfolder}/mod_${CFW_NAME}.txt" ] && source "${controlfolder}/mod_${CFW_NAME}.txt"
get_controls

GAMEDIR="/$directory/ports/zimlite"

cd "$GAMEDIR"

[ -f "$GAMEDIR/log.txt" ] && cp "$GAMEDIR/log.txt" "$GAMEDIR/log.previous.txt"
profile_monitor_pid=''
if [ "${ZIMLITE_PROFILE:-0}" = 1 ] || [ -f "$GAMEDIR/servo/profile.enabled" ]; then
  profile_archive_dir="$GAMEDIR/profiles/$(date -u '+%Y%m%dT%H%M%SZ')-$$"
  export KIWIX_PROFILE_DIR=$(mktemp -d "${TMPDIR:-/tmp}/zimlite-profile.XXXXXX")
  mkdir -p "$KIWIX_PROFILE_DIR"
  exec > >(tee "$KIWIX_PROFILE_DIR/app.log") 2>&1
  sh "$GAMEDIR/servo/profile.sh" "$$" "$KIWIX_PROFILE_DIR" &
  profile_monitor_pid=$!
  echo "Profiling: $KIWIX_PROFILE_DIR"
else
  > "$GAMEDIR/log.txt" && exec > >(tee "$GAMEDIR/log.txt") 2>&1
fi

export SDL_GAMECONTROLLERCONFIG="$sdl_controllerconfig"

$GPTOKEYB "zimlite" &
pm_platform_helper "$GAMEDIR/zimlite"
app_runtime_dir=''
app_binary="$GAMEDIR/zimlite"
if [ -x "$GAMEDIR/servo/run.sh" ]; then
  # Run the Go executable from RAM: concurrent executable page faults and
  # helper startup on the handheld's FUSE/exFAT partition can stall the system.
  app_runtime_dir=$(mktemp -d "${TMPDIR:-/tmp}/zimlite-app.XXXXXX")
  if [ -z "$app_runtime_dir" ] || ! cp "$app_binary" "$app_runtime_dir/zimlite" || ! chmod +x "$app_runtime_dir/zimlite"; then
    echo "Cannot stage Zimlite executable in RAM"
    [ -n "$app_runtime_dir" ] && rm -rf -- "$app_runtime_dir"
    pm_finish
    exit 1
  fi
  export KIWIX_APP_DIR="$GAMEDIR"
  app_binary="$app_runtime_dir/zimlite"
fi
"$app_binary"
[ -n "$app_runtime_dir" ] && rm -rf -- "$app_runtime_dir"

if [ -n "$profile_monitor_pid" ]; then
  kill "$profile_monitor_pid" 2>/dev/null || true
  wait "$profile_monitor_pid" 2>/dev/null || true
  mkdir -p "$profile_archive_dir"
  if cp "$KIWIX_PROFILE_DIR"/* "$profile_archive_dir/" && cp "$KIWIX_PROFILE_DIR/app.log" "$GAMEDIR/log.txt"; then
    rm -rf -- "$KIWIX_PROFILE_DIR"
    echo "Profile archived: $profile_archive_dir"
  else
    echo "Profile archive failed; logs retained at $KIWIX_PROFILE_DIR"
  fi
fi

pm_finish
