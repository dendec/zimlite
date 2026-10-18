#!/bin/sh
# Desktop-only adapter for servo-probe. stdout must contain only the RGBA frame.
set -eu
exec docker run --rm --network host --memory 700m --memory-swap 700m \
  -e LP_NUM_THREADS=2 zimlite-servo-probe \
  /usr/local/bin/zimlite-servo-host "$@"
