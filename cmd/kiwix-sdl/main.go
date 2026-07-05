// Command kiwix-sdl is a lightweight offline ZIM/markdown reader for game consoles.
// Stage 1 MVP: read markdown files and render them via SDL2.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kiwix-sdl/kiwix-sdl/internal/navigation"
	"github.com/kiwix-sdl/kiwix-sdl/internal/renderer"
	"github.com/kiwix-sdl/kiwix-sdl/internal/ui"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var filePath string
	if len(os.Args) >= 2 {
		filePath = os.Args[1]
	} else {
		// Scan current directory for ZIM files
		files, err := os.ReadDir(".")
		if err == nil {
			for _, entry := range files {
				if entry.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext == ".zim" {
					filePath = entry.Name()
					break
				}
			}
		}
		// Fallback to Welcome.md if it exists
		if filePath == "" {
			if _, err := os.Stat("Welcome.md"); err == nil {
				filePath = "Welcome.md"
			}
		}
	}

	if filePath == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s <markdown-file|zim-file>\n", os.Args[0])
		os.Exit(1)
	}

	fontPath := findFont()
	if fontPath == "" {
		fmt.Println("Notice: No external TTF font found. Using embedded fonts.")
	}

	r, err := renderer.New("Kiwix-SDL", 640, 480, fontPath, 18)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating renderer: %v\n", err)
		os.Exit(1)
	}
	defer r.Destroy()

	app := ui.New(r, navigation.NewSimpleNavigator())

	if err := app.OpenFile(filePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}

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
