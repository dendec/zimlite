// Command kiwix-sdl is a lightweight offline ZIM/markdown reader for game consoles.
// Stage 1 MVP: read markdown files and render them via SDL2.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/kiwix-sdl/kiwix-sdl/internal/navigation"
	"github.com/kiwix-sdl/kiwix-sdl/internal/renderer"
	"github.com/kiwix-sdl/kiwix-sdl/internal/ui"
)

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <markdown-file>\n", os.Args[0])
		os.Exit(1)
	}
	filePath := os.Args[1]

	fontPath := findFont()
	if fontPath == "" {
		fmt.Fprintln(os.Stderr, "Error: no TTF font found. Install fonts-dejavu-core or set KIWIX_FONT env var.")
		os.Exit(1)
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
	candidates := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/ubuntu/Ubuntu-R.ttf",
		"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
