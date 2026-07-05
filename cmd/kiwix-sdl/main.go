// Command kiwix-sdl is a lightweight offline ZIM/markdown reader for game consoles.
// Stage 1 MVP: read markdown files and render them via SDL2.
package main

import (
	"log/slog"
	"os"
	"runtime"

	"github.com/kiwix-sdl/kiwix-sdl/internal/config"
	"github.com/kiwix-sdl/kiwix-sdl/internal/navigation"
	"github.com/kiwix-sdl/kiwix-sdl/internal/renderer"
	"github.com/kiwix-sdl/kiwix-sdl/internal/ui"
)

func main() {
	logLevel := slog.LevelInfo
	if os.Getenv("KIWIX_DEBUG") != "" || os.Getenv("KIWIX_DEBUG_INPUT") != "" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var filePath string
	if len(os.Args) >= 2 {
		filePath = os.Args[1]
	} else {
		filePath = "virtual:menu"
	}

	config.Load()
	cfg := config.Get()

	slog.Info("Starting Kiwix-SDL", "filePath", filePath)

	fontPath := findFont()
	if fontPath == "" {
		slog.Info("No external TTF font found. Using embedded fonts.")
	}

	r, err := renderer.New("Kiwix-SDL", 640, 480, fontPath, cfg.FontSize)
	if err != nil {
		slog.Error("Error creating renderer", "error", err)
		os.Exit(1)
	}
	defer r.Destroy()
	slog.Info("Renderer initialized successfully", "font", fontPath)

	if cfg.Theme == "light" && !r.IsLight() {
		r.ToggleTheme()
	} else if cfg.Theme == "dark" && r.IsLight() {
		r.ToggleTheme()
	}

	app := ui.New(r, r, r, navigation.NewSimpleNavigator())

	if err := app.OpenFile(filePath); err != nil {
		slog.Error("Error opening file", "file", filePath, "error", err)
		os.Exit(1)
	}

	slog.Info("Starting application event loop")
	app.Run()
}

func findFont() string {
	if p := os.Getenv("KIWIX_FONT"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
