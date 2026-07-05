#!/bin/bash
DIR=/mnt/SDCARD/Data/ports/kiwix-sdl
export LD_LIBRARY_PATH="$DIR/lib:/usr/lib"
cd "$DIR"
ZIM=$(ls "$DIR"/*.zim 2>/dev/null | head -1)
if [ -n "$ZIM" ]; then
  exec ./kiwix-sdl "$ZIM"
else
  exec ./kiwix-sdl "$@"
fi
