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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dendec/zimlite/internal/article"
	"github.com/dendec/zimlite/internal/profile"
)

const maxEventBytes = 64 << 10
const maxCommandBytes = 8 << 10
const minAvailableMemoryKiB int64 = 50 * 1024

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
	Done       chan struct{}
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
	if available, err := availableMemoryKiB(); err == nil && available < minAvailableMemoryKiB {
		_ = source.Close()
		return nil, fmt.Errorf("insufficient available memory for Servo (%d KiB available; reserve %d KiB)", available, minAvailableMemoryKiB)
	}
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
	session := &Session{Frames: make(chan *image.RGBA, 1), States: make(chan Event, 1), Navigation: make(chan Event, 16), Errors: make(chan error, 1), Done: make(chan struct{}), commands: make(chan []byte, 16), cancel: cancel, base: server.URL("")}
	fail := func(err error) {
		profile.Mark("servo.error", map[string]any{"pid": command.Process.Pid, "error": err.Error()})
		select {
		case session.Errors <- err:
		default:
		}
		cancel()
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
		defer close(session.Done)
		defer func() { _ = source.Close() }()
		defer func() { _ = server.Close() }()
		defer cancel()
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
		if e := command.Wait(); e != nil && ctx.Err() == nil {
			fail(e)
		}
	}()
	go session.watch(ctx, command.Process.Pid, fail)
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
	case <-s.Done:
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
		<-s.Done
		profile.Mark("servo.close.end", nil)
	})
}

// availableMemoryKiB uses reclaimable cache as well as free pages. Unlike RSS,
// this includes system-wide pressure without double-counting shared mappings.
func availableMemoryKiB() (int64, error) {
	mem, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(mem), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "MemAvailable:" && fields[2] == "kB" {
			n, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, err
			}
			if n < 0 {
				return 0, errors.New("invalid MemAvailable")
			}
			return n, nil
		}
	}
	return 0, errors.New("MemAvailable unavailable")
}

// Watchdog supplements resource/IPC budgets with a dynamic system memory reserve.
// This is a sampled guard, not kernel enforcement: sudden allocations can cross
// the reserve between samples. Never use RLIMIT_AS for Servo's large reservations.
func (s *Session) watch(ctx context.Context, pid int, fail func(error)) {
	start := time.Now()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	profile.Mark("servo.watchdog", map[string]any{"pid": pid, "min_available_kib": minAvailableMemoryKiB, "poll_interval_ms": 100, "first_frame_timeout_seconds": 35})
	lastSample := time.Time{}
	reportedUnavailable := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		available, err := availableMemoryKiB()
		if err == nil {
			if available < minAvailableMemoryKiB {
				fields := map[string]any{"pid": pid, "available_kib": available, "min_available_kib": minAvailableMemoryKiB}
				profile.Mark("servo.memory_pressure", fields)
				fail(fmt.Errorf("low system memory (%d KiB available; reserve %d KiB)", available, minAvailableMemoryKiB))
				slog.Warn("Low available memory; switching Servo to SDL", "available_kib", available, "min_available_kib", minAvailableMemoryKiB)
				return
			}
			if time.Since(lastSample) >= time.Second {
				profile.Mark("servo.memory_available", map[string]any{"pid": pid, "available_kib": available, "min_available_kib": minAvailableMemoryKiB})
				lastSample = time.Now()
			}
		} else if !reportedUnavailable {
			profile.Mark("servo.memory_monitor_unavailable", map[string]any{"error": err.Error()})
			reportedUnavailable = true
		}
		if !s.firstFrame.Load() && time.Since(start) > 35*time.Second {
			fail(errors.New("servo first frame timed out"))
			return
		}
	}
}
