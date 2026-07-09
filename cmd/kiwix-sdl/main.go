// Command kiwix-sdl is a lightweight offline ZIM/markdown reader for game consoles.
// Stage 1 MVP: read markdown files and render them via SDL2.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/kiwix-sdl/kiwix-sdl/internal/config"
	"github.com/kiwix-sdl/kiwix-sdl/internal/navigation"
	"github.com/kiwix-sdl/kiwix-sdl/internal/renderer"
	"github.com/kiwix-sdl/kiwix-sdl/internal/storage"
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
		if os.Args[1] == "--version" || os.Args[1] == "-v" {
			fmt.Printf("Kiwix-SDL %s\n", storage.Version)
			return
		}
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

	r, err := renderer.New(fmt.Sprintf("Kiwix-SDL %s", storage.Version), 640, 480, fontPath, cfg.FontSize)
	if err != nil {
		slog.Error("Error creating renderer", "error", err)
		// On Windows,Stderr might not be visible easily, so we show a message box if possible
		// (though slog should have written to the 2> file)
		os.Exit(1)
	}
	defer r.Destroy()
	slog.Info("Renderer initialized successfully", "font", fontPath, "renderer", "sdl")

	if cfg.Theme == "light" && !r.IsLight() {
		r.ToggleTheme()
	} else if cfg.Theme == "dark" && r.IsLight() {
		r.ToggleTheme()
	}

	app := ui.New(r, r, r, navigation.NewSimpleNavigator(), config.NewProvider())

	if err := app.OpenFile(filePath); err != nil {
		slog.Error("Error opening file", "file", filePath, "error", err)
		// Try to fallback to menu if requested file failed
		if filePath != "virtual:menu" {
			slog.Info("Falling back to menu")
			if err := app.OpenFile("virtual:menu"); err != nil {
				slog.Error("Critical error: menu fallback failed", "error", err)
				os.Exit(1)
			}
		} else {
			os.Exit(1)
		}
	}

	slog.Info("Starting application event loop")
	app.Run()
	slog.Info("Application exited normally")
}

func findFont() string {
	if p := os.Getenv("KIWIX_FONT"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
