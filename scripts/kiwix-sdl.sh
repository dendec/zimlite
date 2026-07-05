#!/bin/bash
DIR=/mnt/SDCARD/Data/ports/kiwix-sdl
export LD_LIBRARY_PATH="$DIR/lib:/usr/lib"
cd "$DIR"
exec ./kiwix-sdl "$@"
