#!/bin/sh
# Independent process: continue recording if the application's UI thread stalls.
# Usage: profile.sh LAUNCHER_PID RAM_DIRECTORY [ARCHIVE_DIRECTORY]
set -eu

# A separate worker makes bounded crash snapshots without putting SD-card
# writes on the UI or system sampler. Never spawn overlapping snapshot workers.
if [ "${1:-}" = --snapshot ]; then
    snapshot_parent=$2
    snapshot_ram=$3
    snapshot_archive=$4
    while [ -d "/proc/$snapshot_parent" ]; do
        for snapshot_name in events.jsonl app.log system.log kernel.log; do
            [ -f "$snapshot_ram/$snapshot_name" ] || continue
            snapshot_tmp="$snapshot_archive/$snapshot_name.live.tmp"
            if tail -c 262144 "$snapshot_ram/$snapshot_name" > "$snapshot_tmp"; then
                mv -f "$snapshot_tmp" "$snapshot_archive/$snapshot_name.live"
            fi
        done
        sleep 2
    done
    exit 0
fi

launcher_pid=$1
profile_dir=$2
mkdir -p "$profile_dir"
snapshot_worker=''
if [ -n "${3:-}" ]; then
    sh "$0" --snapshot "$$" "$profile_dir" "$3" &
    snapshot_worker=$!
fi
# Do not wait for a worker stuck in filesystem I/O on shutdown.
trap '[ -z "$snapshot_worker" ] || kill "$snapshot_worker" 2>/dev/null || true' EXIT
trap 'exit 0' HUP INT TERM
sample_log="$profile_dir/system.log"
kernel_log="$profile_dir/kernel.log"
dmesg > "$kernel_log" 2>&1 || true
sample=0
while [ -d "/proc/$launcher_pid" ]; do
    {
        printf '\nSAMPLE %s uptime=' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
        cat /proc/uptime
        awk '/^(MemTotal|MemFree|MemAvailable|Buffers|Cached|SwapCached|Active|Inactive|AnonPages|Mapped|Shmem|Slab|SReclaimable|SUnreclaim|KernelStack|PageTables|SwapTotal|SwapFree|Dirty|Writeback|CmaTotal|CmaFree):/' /proc/meminfo
        awk '/^(pgmajfault|pswpin|pswpout|pgscan|pgsteal|allocstall|oom_kill|nr_free_pages|nr_anon_pages|nr_file_pages|nr_slab_unreclaimable)/' /proc/vmstat
        head -n 1 /proc/stat
        for pressure in /proc/pressure/memory /proc/pressure/io; do
            if [ -r "$pressure" ]; then printf 'PRESSURE %s\n' "$pressure"; cat "$pressure"; fi
        done
        for comm_file in /proc/[0-9]*/comm; do
            [ -r "$comm_file" ] || continue
            process_name=''
            read -r process_name < "$comm_file" || continue
            case "$process_name" in
                zimlite|zimlite-servo-h*) ;;
                *) continue ;;
            esac
            process_dir=${comm_file%/comm}
            printf 'PROCESS %s %s\n' "${process_dir##*/}" "$process_name"
            awk '/^(State|VmPeak|VmSize|VmHWM|VmRSS|RssAnon|RssFile|RssShmem|VmData|VmStk|VmSwap|Threads):/' "$process_dir/status" 2>/dev/null || true
            # Includes user/system CPU ticks and minor/major page faults.
            cat "$process_dir/stat" 2>/dev/null || true
            printf 'WCHAN '; cat "$process_dir/wchan" 2>/dev/null || true; printf '\n'
            if [ "$((sample % 5))" = 0 ]; then
                if [ -r "$process_dir/smaps_rollup" ]; then
                    awk '/^(Rss|Pss|Private_Clean|Private_Dirty|Swap|SwapPss):/' "$process_dir/smaps_rollup" 2>/dev/null || true
                fi
                for thread in "$process_dir"/task/[0-9]*; do
                    [ -r "$thread/wchan" ] || continue
                    printf 'THREAD %s ' "${thread##*/}"; cat "$thread/wchan" 2>/dev/null || true; printf '\n'
                done
            fi
        done
    } >> "$sample_log"
    if [ "$((sample % 5))" = 0 ]; then
        dmesg > "$kernel_log.tmp" 2>&1 || true
        mv -f "$kernel_log.tmp" "$kernel_log"
    fi
    # Do not synchronously flush FUSE/exFAT while the renderer is active.
    if [ "$(wc -c < "$sample_log")" -gt 2097152 ]; then
        mv -f "$sample_log" "$profile_dir/system.previous.log"
    fi
    sample=$((sample + 1))
    sleep 1
done
