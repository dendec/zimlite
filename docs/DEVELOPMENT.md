# Development & Architecture

## Project Architecture

```
cmd/kiwix-sdl/main.go    — Entry point, wires dependencies
internal/
  config/                — JSON config (theme, font, lang)
  document/              — Document model (Blocks/Inlines/Visitor)
  markdown/              — goldmark AST → Document
  html/                  — HTML → Markdown → Document + Math plugin
  zim/                   — libzim C++ bridge (cgo)
  renderer/              — SDL2 rendering (layout, draw, fonts, themes, images, emoji)
  ui/                    — App loop, input, loader, library browser, gamepad
  menu/                  — Virtual pages: file menu, help, settings
  navigation/            — History stack for back navigation
  storage/               — File I/O, ZIM opening, HTTP download
  trie/                  — Radix tree for ZIM article tree
  svg/                   — Embedded LunaSVG for SVG rasterization
portmaster/              — PortMaster distribution config
```

## Build Targets

| Target | Arch | Platform |
|---|---|---|
| `make build` | x86_64 | Linux native |
| `make build-linux-amd64` | x86_64 | Linux cross |
| `make build-linux-arm64` | aarch64 | Linux cross |
| `make build-linux-armv8` | armv7l | Linux cross |
| `make build-windows-amd64` | x86_64 | Windows (Docker) |

## Dependencies

- **Go** 1.25+
- **SDL2**, **SDL2_ttf** (system or cross-compiled)
- **libzim** 9.7 (auto-downloaded or cross-built)
- **liblzma**, **libzstd**, **libicu** (for libzim)

Go modules: `go-sdl2`, `goldmark`, `html-to-markdown`, `golang.org/x/image` (webp), `golang.org/x/net`
