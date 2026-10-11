#!/bin/sh
# One-shot hardware probe only. Firmware without memory cgroups uses an RSS
# watchdog; it cannot account for GPU allocations or guarantee no overshoot.
set -eu
helper_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
"$helper_dir/zimlite-servo-host" "$@" &
helper_pid=$!
monitor_pid=
cleanup() {
    kill "$helper_pid" 2>/dev/null || true
    if [ -n "$monitor_pid" ]; then kill "$monitor_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT HUP INT TERM
(
    ticks=0
    while kill -0 "$helper_pid" 2>/dev/null; do
        usage=$(awk '/^VmRSS:|^VmSwap:/{sum+=$2}END{print sum+0}' "/proc/$helper_pid/status" 2>/dev/null || echo 0)
        if [ "$usage" -gt 512000 ]; then
            echo "hardware probe exceeded 500 MiB RSS+swap; stopping helper" >&2
            kill -KILL "$helper_pid" 2>/dev/null || true
            exit
        fi
        ticks=$((ticks+1))
        if [ "$ticks" -ge 350 ]; then
            echo "hardware probe exceeded 35 seconds; stopping helper" >&2
            kill -KILL "$helper_pid" 2>/dev/null || true
            exit
        fi
        sleep 0.1
    done
) &
monitor_pid=$!
wait "$helper_pid"
