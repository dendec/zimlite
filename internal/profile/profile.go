// Package profile records opt-in diagnostics independently of the UI logger.
package profile

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var recorder struct {
	sync.Mutex
	file  *os.File
	bytes int
}

// Start enables diagnostics when KIWIX_PROFILE_DIR is set. Each run gets its
// own directory from the launcher. Avoid synchronous FUSE writes in the UI,
// resource server and helper startup paths; the launcher uses temporary RAM logs.
func Start() func() {
	dir := os.Getenv("KIWIX_PROFILE_DIR")
	if dir == "" {
		return func() {}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("Cannot create profile directory", "error", err)
		return func() {}
	}
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("Cannot open profile log", "error", err)
		return func() {}
	}
	recorder.Lock()
	recorder.file = f
	recorder.bytes = 0
	recorder.Unlock()
	Mark("app.start", map[string]any{"pid": os.Getpid(), "go": runtime.Version()})
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				Mark("go.memory", map[string]any{"heap_alloc": m.HeapAlloc, "heap_sys": m.HeapSys, "heap_inuse": m.HeapInuse, "heap_idle": m.HeapIdle, "heap_released": m.HeapReleased, "sys": m.Sys, "stack_inuse": m.StackInuse, "gc_count": m.NumGC, "gc_pause_total_ns": m.PauseTotalNs, "goroutines": runtime.NumGoroutine(), "cgo_calls": runtime.NumCgoCall()})
			}
		}
	}()
	return func() {
		close(stop)
		<-done
		Mark("app.stop", nil)
		recorder.Lock()
		defer recorder.Unlock()
		_ = f.Close()
		recorder.file = nil
	}
}

// Mark records a bounded event. Paths and timings are logged; article bodies,
// cookies, authentication tokens and process environments are never recorded.
func Mark(event string, fields map[string]any) {
	recorder.Lock()
	defer recorder.Unlock()
	if recorder.file == nil || recorder.bytes >= 16<<20 {
		return
	}
	if fields == nil {
		fields = make(map[string]any)
	}
	fields["event"] = event
	fields["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.Marshal(fields)
	if err != nil || len(data) > 16<<10 {
		return
	}
	data = append(data, '\n')
	n, err := recorder.file.Write(data)
	recorder.bytes += n
	_ = err
}
