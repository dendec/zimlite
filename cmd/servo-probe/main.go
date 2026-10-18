// Command servo-probe captures one article viewport with the experimental Servo helper.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dendec/zimlite/internal/article"
	"github.com/dendec/zimlite/internal/profile"
	"github.com/dendec/zimlite/internal/servo"
)

func main() {
	stopProfile := profile.Start()
	defer stopProfile()
	if err := run(); err != nil {
		slog.Error("Servo probe failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	sourcePath := flag.String("source", "", "HTML file or ZIM archive")
	articlePath := flag.String("article", "", "optional entry path inside a ZIM archive")
	helper := flag.String("helper", "./zimlite-servo-host", "path to Servo helper")
	output := flag.String("output", "servo-probe.png", "output PNG")
	width := flag.Int("width", 640, "viewport width")
	height := flag.Int("height", 456, "viewport height excluding status bar")
	allowMissing := flag.Bool("allow-missing-resources", false, "save diagnostic frame even when archive resources are unavailable")
	scrollDuration := flag.Duration("scroll-duration", 0, "profile continuous scrolling for up to 30s instead of saving a PNG")
	scrollPause := flag.Duration("scroll-pause", 0, "pause after each 2s of scroll commands to profile raster transitions (0..2s)")
	scrollFrames := flag.String("scroll-frames-dir", "", "optional directory for at most five raster transition PNGs, saved after stopping the helper")
	flag.Parse()
	if *sourcePath == "" {
		return errors.New("--source is required")
	}
	if *width <= 0 || *height <= 0 || int64(*width)*int64(*height) > servo.MaxPixels {
		return errors.New("viewport exceeds memory budget")
	}
	if *scrollDuration < 0 || *scrollDuration > 30*time.Second {
		return errors.New("scroll profile duration must be between 0 and 30s")
	}
	if *scrollPause < 0 || *scrollPause > 2*time.Second {
		return errors.New("scroll pause must be between 0 and 2s")
	}
	var source article.Source
	var entry string
	var err error
	if strings.EqualFold(filepath.Ext(*sourcePath), ".zim") {
		source, entry, err = article.OpenZIMSource(*sourcePath)
		if *articlePath != "" {
			entry = *articlePath
		}
	} else {
		source, entry, err = article.OpenFileSource(*sourcePath)
	}
	if err != nil {
		return err
	}
	if *scrollDuration > 0 {
		return profileScroll(source, entry, *helper, uint32(*width), uint32(*height), *scrollDuration, *scrollPause, *scrollFrames)
	}
	defer func() { _ = source.Close() }()
	server, err := article.StartServer(source)
	if err != nil {
		return err
	}
	defer func() { _ = server.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, *helper, server.URL(entry), strconv.Itoa(*width), strconv.Itoa(*height))
	command.Stderr = os.Stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	started := time.Now()
	if err := command.Start(); err != nil {
		return fmt.Errorf("start Servo helper: %w", err)
	}
	frame, frameErr := servo.ReadFrame(stdout)
	if frameErr != nil {
		_ = command.Process.Kill()
	}
	waitErr := command.Wait()
	if frameErr != nil {
		return fmt.Errorf("read Servo frame: %w", frameErr)
	}
	if waitErr != nil {
		return fmt.Errorf("servo helper: %w", waitErr)
	}
	if server.Failures() != 0 {
		if !*allowMissing {
			return fmt.Errorf("article had %d unavailable or over-budget resources", server.Failures())
		}
		slog.Warn("Saving incomplete diagnostic frame", "resource_failures", server.Failures())
	}
	f, err := os.Create(*output)
	if err != nil {
		return err
	}
	encodeErr := png.Encode(f, frame)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	slog.Info("Servo frame captured", "output", *output, "elapsed", time.Since(started))
	return nil
}

// Session retains the normal watchdog/resource budgets during performance probes.
func profileScroll(source article.Source, entry, helper string, width, height uint32, duration, pause time.Duration, frameDir string) error {
	session, err := servo.NewSession(helper, source, entry, width, height)
	if err != nil {
		return err
	}
	defer session.Close()
	firstFrame := time.NewTimer(25 * time.Second)
	defer firstFrame.Stop()
	var captures []*image.RGBA
	capture := func(frame *image.RGBA) {
		if frameDir == "" || len(captures) >= 5 {
			return
		}
		if len(captures) == 0 || frame.Bounds() != captures[len(captures)-1].Bounds() {
			captures = append(captures, frame)
		}
	}
	select {
	case frame := <-session.Frames:
		capture(frame)
	case err := <-session.Errors:
		return err
	case <-firstFrame.C:
		return errors.New("first frame timed out during scroll profile")
	}
	started, previous := time.Now(), time.Now()
	end := time.NewTimer(duration)
	defer end.Stop()
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()
	frames, commands, scrollY := 0, 0, int32(0)
	var maxGap time.Duration
	var pauseUntil time.Time
	for {
		select {
		case now := <-ticker.C:
			if now.Before(pauseUntil) {
				continue
			}
			// Reverse every two seconds to keep measuring motion instead of
			// sitting at the bottom of a finite article.
			delta := int32(24)
			if (commands/60)%2 != 0 {
				delta = -delta
			}
			if err := session.Send(servo.Command{Op: "scroll", Value: delta}); err != nil {
				return err
			}
			commands++
			if commands%60 == 0 {
				pauseUntil = now.Add(pause)
			}
		case frame := <-session.Frames:
			capture(frame)
			now := time.Now()
			maxGap = max(maxGap, now.Sub(previous))
			previous = now
			frames++
		case state := <-session.States:
			scrollY = state.ScrollY
		case err := <-session.Errors:
			return err
		case <-end.C:
			seconds := time.Since(started).Seconds()
			slog.Info("Servo scroll profile", "entry", entry, "width", width, "height", height, "seconds", seconds, "frames", frames, "fps", float64(frames)/seconds, "max_frame_gap_ms", maxGap.Milliseconds(), "commands", commands, "scroll_y", scrollY, "pause_ms", pause.Milliseconds())
			// Keep consuming frames after release so the helper can restore
			// full raster resolution without stdout backpressure.
			settle := time.NewTimer(3 * time.Second)
			defer settle.Stop()
			for {
				select {
				case frame := <-session.Frames:
					capture(frame)
				case <-session.States:
				case err := <-session.Errors:
					return err
				case <-settle.C:
					session.Close()
					return saveScrollFrames(frameDir, captures)
				}
			}
		}
	}
}

// Encode only after the helper has stopped, keeping profiling free of PNG work
// and avoiding extra SD-card writes while the device is under render load.
func saveScrollFrames(dir string, frames []*image.RGBA) error {
	if len(frames) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for i, frame := range frames {
		path := filepath.Join(dir, fmt.Sprintf("transition-%02d-%dx%d.png", i, frame.Bounds().Dx(), frame.Bounds().Dy()))
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		encodeErr := png.Encode(f, frame)
		closeErr := f.Close()
		if encodeErr != nil {
			return encodeErr
		}
		if closeErr != nil {
			return closeErr
		}
		slog.Info("Servo transition frame captured", "output", path)
	}
	return nil
}
