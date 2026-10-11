// Command servo-probe captures one article viewport with the experimental Servo helper.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
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
	flag.Parse()
	if *sourcePath == "" {
		return errors.New("--source is required")
	}
	if *width <= 0 || *height <= 0 || int64(*width)*int64(*height) > servo.MaxPixels {
		return errors.New("viewport exceeds memory budget")
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
