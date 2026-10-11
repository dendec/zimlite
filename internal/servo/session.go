package servo

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dendec/zimlite/internal/article"
	"github.com/dendec/zimlite/internal/memory"
	"github.com/dendec/zimlite/internal/profile"
)

const maxEventBytes = 64 << 10
const maxCommandBytes = 8 << 10

// Keep at most one helper/source alive, including deferred storage cleanup.
var activeSessions atomic.Int32

// Command contains semantic viewer operations, never arbitrary scripts.
type Command struct {
	Op     string  `json:"op"`
	Value  int32   `json:"value,omitempty"`
	X      int32   `json:"x,omitempty"`
	Y      int32   `json:"y,omitempty"`
	Width  uint32  `json:"width,omitempty"`
	Height uint32  `json:"height,omitempty"`
	Scale  float32 `json:"scale,omitempty"`
	Text   string  `json:"text,omitempty"`
}

type Event struct {
	Event            string `json:"event"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	ScrollY          int32  `json:"scroll_y"`
	LinkCount        int    `json:"link_count"`
	SelectedLink     int    `json:"selected_link"`
	SelectedURL      string `json:"selected_url"`
	TableScrollLeft  bool   `json:"table_scroll_left"`
	TableScrollRight bool   `json:"table_scroll_right"`
}

// Session owns one helper, resource server and Source. NewSession takes Source
// ownership even when startup fails. Channels are bounded; old frames and state
// are replaced when SDL is busy. Navigation is kept in a separate bounded queue.
type Session struct {
	Frames     chan *image.RGBA
	States     chan Event
	Navigation chan Event
	Errors     chan error
	Done       chan struct{} // Resource cleanup completed.
	stopped    chan struct{} // Helper exited and HTTP connections were aborted.
	commands   chan []byte
	cancel     context.CancelFunc
	closeOnce  sync.Once
	firstFrame atomic.Bool
	base       string
}

func NewSession(helper string, source article.Source, entry string, width, height uint32) (*Session, error) {
	started := time.Now()
	profile.Mark("servo.start.begin", map[string]any{"entry": entry, "width": width, "height": height})
	if width == 0 || height == 0 || uint64(width)*uint64(height) > MaxPixels || width > 1280 || height > 1280 {
		_ = source.Close()
		return nil, errors.New("servo viewport exceeds memory budget")
	}
	if activeSessions.Load() != 0 {
		_ = source.Close()
		return nil, errors.New("previous Servo resource read is still stopping")
	}
	budget := memoryBudget{reserveKiB: 128 * 1024, helperKiB: 512 * 1024, helperSwapKiB: 16 * 1024}
	if system, e := memory.ReadSystem(); e == nil {
		budget = budgetFor(system)
		if budget.helperKiB < minimumHelperKiB {
			_ = source.Close()
			return nil, fmt.Errorf("insufficient RAM for Servo startup (%d KiB helper budget; need %d KiB)", budget.helperKiB, minimumHelperKiB)
		}
	} else {
		slog.Warn("Servo physical memory monitor unavailable", "error", e)
	}
	if !activeSessions.CompareAndSwap(0, 1) {
		_ = source.Close()
		return nil, errors.New("another Servo session is still stopping")
	}
	helperStarted := false
	defer func() {
		if !helperStarted {
			activeSessions.Add(-1)
		}
	}()
	server, err := article.StartServer(source)
	if err != nil {
		_ = source.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, helper, "--session", server.URL(entry), strconv.FormatUint(uint64(width), 10), strconv.FormatUint(uint64(height), 10))
	command.Stderr = os.Stderr
	command.WaitDelay = time.Second
	configureProcess(command)
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		_ = server.Close()
		_ = source.Close()
		return nil, err
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		_ = server.Close()
		_ = source.Close()
		return nil, err
	}
	if err = command.Start(); err != nil {
		cancel()
		_ = server.Close()
		_ = source.Close()
		return nil, fmt.Errorf("start Servo: %w", err)
	}
	helperStarted = true
	cancelAll := func() { server.Stop(); cancel(); _ = stdout.Close() }
	session := &Session{Frames: make(chan *image.RGBA, 1), States: make(chan Event, 1), Navigation: make(chan Event, 16), Errors: make(chan error, 1), Done: make(chan struct{}), commands: make(chan []byte, 16), cancel: cancelAll, stopped: make(chan struct{}), base: server.URL("")}
	prioritizeHelperOOM(command.Process.Pid)
	var failureOnce sync.Once
	fail := func(err error) {
		failureOnce.Do(func() {
			profile.Mark("servo.error", map[string]any{"pid": command.Process.Pid, "error": err.Error()})
			select {
			case session.Errors <- err:
			default:
			}
			cancelAll()
		})
	}
	go func() {
		defer func() { _ = stdin.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case data := <-session.commands:
				var size [4]byte
				binary.LittleEndian.PutUint32(size[:], uint32(len(data)))
				if _, e := stdin.Write(size[:]); e != nil {
					if ctx.Err() == nil {
						fail(e)
					}
					return
				}
				if _, e := stdin.Write(data); e != nil {
					if ctx.Err() == nil {
						fail(e)
					}
					return
				}
			}
		}
	}()
	go func() {
		for {
			frame, event, e := readPacket(stdout)
			if e != nil {
				if ctx.Err() == nil {
					fail(fmt.Errorf("read Servo: %w", e))
				}
				break
			}
			if frame != nil {
				if !session.firstFrame.Swap(true) {
					profile.Mark("servo.first_frame", map[string]any{"pid": command.Process.Pid, "elapsed_ms": time.Since(started).Milliseconds(), "width": frame.Rect.Dx(), "height": frame.Rect.Dy()})
				}
				select {
				case session.Frames <- frame:
				default:
					select {
					case <-session.Frames:
					default:
					}
					select {
					case session.Frames <- frame:
					default:
					}
				}
			} else if event.Event == "state" {
				select {
				case session.States <- event:
				default:
					select {
					case <-session.States:
					default:
					}
					select {
					case session.States <- event:
					default:
					}
				}
			} else if event.Event == "navigate" {
				select {
				case session.Navigation <- event:
				default:
					fail(errors.New("servo navigation queue exceeded budget"))
				}
			} else {
				fail(errors.New("unsupported Servo event"))
				break
			}
		}
		waitErr := command.Wait()
		if waitErr != nil && ctx.Err() == nil {
			fail(waitErr)
		}
		cancel()
		server.Stop()
		fields := map[string]any{"pid": command.Process.Pid}
		if waitErr != nil {
			fields["exit_error"] = waitErr.Error()
		}
		profile.Mark("servo.helper.exited", fields)
		go func() {
			defer close(session.Done)
			defer activeSessions.Add(-1)
			_ = server.Close()
			_ = source.Close()
			profile.Mark("servo.resources.closed", map[string]any{"pid": command.Process.Pid})
		}()
		// Normal reads finish immediately. A blocked storage read must not
		// hold the SDL thread while the helper's memory has already been freed.
		select {
		case <-session.Done:
		case <-time.After(10 * time.Millisecond):
		}
		close(session.stopped)
	}()
	go session.watch(ctx, command.Process.Pid, budget, server.Errors, fail)
	profile.Mark("servo.started", map[string]any{"pid": command.Process.Pid})
	return session, nil
}

func readPacket(r io.Reader) (*image.RGBA, Event, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, Event{}, err
	}
	if string(magic[:]) == "ZSR1" {
		frame, err := ReadFrame(io.MultiReader(bytes.NewReader(magic[:]), r))
		return frame, Event{}, err
	}
	if string(magic[:]) != "ZSE1" {
		return nil, Event{}, errors.New("unsupported Servo packet")
	}
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, Event{}, err
	}
	n := binary.LittleEndian.Uint32(header[:])
	if n == 0 || n > maxEventBytes {
		return nil, Event{}, errors.New("servo event exceeds memory budget")
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, Event{}, err
	}
	var event Event
	err := json.Unmarshal(data, &event)
	return nil, event, err
}

// Send never blocks SDL on a pipe. Overflow terminates the helper so stale input
// cannot build an unlimited queue or keep consuming memory in the background.
func (s *Session) Send(command Command) error {
	data, err := json.Marshal(command)
	if err != nil {
		return err
	}
	if len(data) > maxCommandBytes {
		return errors.New("servo command exceeds memory budget")
	}
	select {
	case <-s.stopped:
		return errors.New("servo session closed")
	default:
	}
	select {
	case s.commands <- data:
		return nil
	default:
		s.cancel()
		return errors.New("servo input queue exceeded budget")
	}
}
func (s *Session) BaseURL() string { return s.base }
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		profile.Mark("servo.close.begin", nil)
		s.cancel()
		select {
		case <-s.stopped:
		case <-time.After(500 * time.Millisecond):
			profile.Mark("servo.close.deferred", nil)
			return // Never block SDL on an uninterruptible kernel/storage wait.
		}
		// The producer has stopped; release queued RGBA and control data.
		drain(s.Frames)
		drain(s.States)
		drain(s.Navigation)
		drain(s.commands)
		profile.Mark("servo.close.end", nil)
	})
}

func drain[T any](queue chan T) {
	for {
		select {
		case <-queue:
		default:
			return
		}
	}
}

// watch samples physical memory before reclaim or swap makes the device
// unresponsive. Helper virtual address space is deliberately not limited.
func (s *Session) watch(ctx context.Context, pid int, budget memoryBudget, resourceErrors <-chan error, fail func(error)) {
	start := time.Now()
	tick := time.NewTicker(memoryPollInterval)
	defer tick.Stop()
	fields := map[string]any{"pid": pid, "reserve_kib": budget.reserveKiB, "helper_budget_kib": budget.helperKiB, "helper_swap_limit_kib": budget.helperSwapKiB, "poll_interval_ms": memoryPollInterval.Milliseconds(), "first_frame_timeout_seconds": firstFrameTimeout.Seconds()}
	profile.Mark("servo.watchdog", fields)
	slog.Info("Servo memory guard", "reserve_kib", budget.reserveKiB, "helper_budget_kib", budget.helperKiB, "helper_swap_limit_kib", budget.helperSwapKiB)
	var previous memory.System
	previousTime, lastSample := time.Time{}, time.Time{}
	reportedUnavailable := false
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-resourceErrors:
			fail(err)
			return
		case <-tick.C:
		}
		now := time.Now()
		system, err := memory.ReadSystem()
		helper, helperErr := memory.ReadProcess(pid)
		if err == nil {
			// Helper RSS is optional during exec/exit; system reserve still applies.
			if helperErr != nil {
				helper = memory.Process{}
			}
			fields := map[string]any{"pid": pid, "available_kib": system.AvailableKiB, "total_kib": system.TotalKiB, "swap_free_kib": system.SwapFreeKiB, "helper_rss_kib": helper.RSSKiB, "helper_swap_kib": helper.SwapKiB, "helper_peak_kib": helper.PeakKiB, "reserve_kib": budget.reserveKiB, "helper_budget_kib": budget.helperKiB, "first_frame": s.firstFrame.Load()}
			if e := budget.check(system, helper, previous, now.Sub(previousTime)); e != nil {
				fields["reason"] = e.Error()
				profile.Mark("servo.memory_pressure", fields)
				fail(e)
				slog.Warn("Stopping Servo before memory exhaustion", "reason", e, "available_kib", system.AvailableKiB, "helper_rss_kib", helper.RSSKiB, "helper_swap_kib", helper.SwapKiB)
				return
			}
			previous, previousTime = system, now
			if now.Sub(lastSample) >= 250*time.Millisecond {
				profile.Mark("servo.memory_sample", fields)
				lastSample = now
			}
		} else if !reportedUnavailable {
			profile.Mark("servo.memory_monitor_unavailable", map[string]any{"error": err.Error()})
			reportedUnavailable = true
		}
		if err != nil && helperErr == nil {
			if e := budget.checkHelper(helper); e != nil {
				fail(e)
				return
			}
		}
		if !s.firstFrame.Load() && now.Sub(start) > firstFrameTimeout {
			fail(errors.New("servo first frame timed out"))
			return
		}
	}
}
