#!/bin/bash
DIR=/mnt/SDCARD/Data/ports/zimlite
export LD_LIBRARY_PATH="$DIR/lib:/usr/lib"
cd "$DIR"
exec ./zimlite "$@"
