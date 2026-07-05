#!/bin/bash
# PORTMASTER: kiwix-sdl.zip, Kiwix SDL.sh
# PortMaster launch script for kiwix-sdl

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

GAMEDIR="/$directory/ports/kiwix-sdl"
cd "$GAMEDIR"

export LD_LIBRARY_PATH="$GAMEDIR/lib:/usr/lib:$LD_LIBRARY_PATH"

exec > >(tee "$GAMEDIR/log.txt") 2>&1

ZIM=$(ls "$GAMEDIR"/*.zim 2>/dev/null | head -1)
if [ -n "$ZIM" ]; then
  exec ./kiwix-sdl "$ZIM"
else
  exec ./kiwix-sdl
fi
