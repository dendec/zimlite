package textinput

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/dendec/zimlite/internal/memory"
)

type Result struct{ Title, Path string }
type Results struct {
	Entries []Result
	More    bool
	Err     error
}

// WatchWorker keeps a physical-memory reserve; large mmaps are not treated as RSS.
func WatchWorker(done <-chan struct{}, kill func()) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if m, err := memory.ReadSystem(); err == nil && m.AvailableKiB < 50*1024 {
				kill()
				return
			}
		}
	}
}

// diagnosticBuffer bounds library diagnostics without blocking the worker.
type diagnosticBuffer struct{ data []byte }

func (b *diagnosticBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if remaining := 4096 - len(b.data); remaining > 0 {
		b.data = append(b.data, p[:min(remaining, n)]...)
	}
	return n, nil
}
func Search(ctx context.Context, archive, query string, offset int) Results {
	if len(query) == 0 || len(query) > MaxTextBytes || offset < 0 || offset > 10000 {
		return Results{Err: fmt.Errorf("invalid search request")}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	cmd := WorkerCommand("zimlite-search-host", archive, query, strconv.Itoa(offset))
	var diagnostic diagnosticBuffer
	cmd.Stderr = &diagnostic
	out, err := cmd.StdoutPipe()
	if err != nil {
		return Results{Err: err}
	}
	if err = cmd.Start(); err != nil {
		return Results{Err: err}
	}
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
			killWorker(cmd)
		case <-finished:
		}
	}()
	go WatchWorker(finished, func() { killWorker(cmd) })
	var result Results
	complete := false
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) == 3 && fields[0] == "R" && len(result.Entries) < 10 {
			title, e := decode(fields[1])
			if e != nil {
				result.Err = e
				break
			}
			path, e := decode(fields[2])
			if e != nil {
				result.Err = e
				break
			}
			result.Entries = append(result.Entries, Result{title, path})
		} else if len(fields) == 2 && fields[0] == "D" {
			complete = true
			result.More = fields[1] == "1"
		} else if len(fields) == 2 && fields[0] == "E" {
			message, e := decode(fields[1])
			if e != nil {
				result.Err = e
			} else {
				result.Err = fmt.Errorf("%s", message)
			}
			break
		} else {
			result.Err = fmt.Errorf("invalid search response: %.160q", scanner.Text())
			break
		}
	}
	if result.Err != nil {
		killWorker(cmd)
	}
	waitErr := cmd.Wait()
	if len(diagnostic.data) > 0 {
		slog.Info("FTS worker diagnostic", "message", strings.TrimSpace(string(diagnostic.data)))
	}
	if result.Err == nil {
		result.Err = scanner.Err()
	}
	if result.Err == nil {
		result.Err = waitErr
	}
	if result.Err == nil && !complete {
		result.Err = fmt.Errorf("incomplete search response")
	}
	if ctx.Err() != nil {
		result.Err = ctx.Err()
	}
	slog.Info("FTS search completed", "query_bytes", len(query), "offset", offset, "results", len(result.Entries), "duration", time.Since(started), "error", result.Err)
	return result
}
