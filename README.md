# Kiwix SDL

**Lightweight offline ZIM/Markdown reader for game consoles and desktop.**

Renders Wikipedia ZIM archives, Markdown, and HTML files via SDL2. Supports keyboard, mouse, and gamepad input. Designed for low-power ARM devices (PortMaster) but runs on any Linux/Windows desktop.

<img width="420" alt="article" src="https://github.com/user-attachments/assets/06cb4988-8b77-443f-953e-ee65e7fd6d14" />

<details>
  <summary><strong>More Screenshots</strong>
</summary>
  <br>
  <img width="420" alt="menu" src="https://github.com/user-attachments/assets/816e3bb0-f05d-4ac2-9952-8df31303a4fa" />
  <img width="420" alt="library" src="https://github.com/user-attachments/assets/83c83d73-9d1c-4344-b67c-c24fdb7ec8ec" />
  <img width="420" alt="tree" src="https://github.com/user-attachments/assets/43edfc1a-7d73-4200-8390-776d76080c26" />
  <img width="420" alt="cn_math" src="https://github.com/user-attachments/assets/2b09733e-3093-4ee9-a35e-f2fb63a169f4" />

</details>

## Features

- **HTML** — Converts HTML to Markdown then render it
- **Article Tree** — Radix-tree browser for ZIM articles
- **Online Library** — Browse & download ZIM archives from [Kiwix Library](https://browse.library.kiwix.org/#lang=)
- **Emoji** — Embedded Twemoji SVGs (compressed zip), rendered via LunaSVG
- **Animated GIFs** — Frame-based animation in documents
- **SVG Images** — Inline SVG rasterization
- **Themes** — Light/Dark color schemes
- **Font Zoom** — Adjustable font size
- **Settings** — Persisted to `config.json`
- **Gamepad** — Full controller support for PortMaster devices
- **Touch/Mouse** — Click links, scroll wheels

## Quick Start

### Linux (native)

```bash
# Install dependencies
sudo apt install libsdl2-dev libsdl2-ttf-dev liblzma-dev libzstd-dev libicu-dev

# Build
make

# Run with a ZIM archive or markdown file
./kiwix-sdl wikipedia_ru_top_mini_2026-04.zim
./kiwix-sdl test.md
```

### Windows (cross-build)

```bash
make build-windows-amd64  # requires Docker
# Output in dist/windows/
```

### ARM64 / PortMaster

```bash
make dist-arm64      # Docker cross-build
make deploy          # Push via ADB to device
make dist-portmaster # Package for PortMaster
```

## Development

For architecture details, build targets, and dependencies, see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

## Configuration

`config.json` next to the binary:

```json
{
  "language": "en",
  "theme": "dark",
  "font_size": 16
}
```

Set via `KIWIX_FONT` env var for a custom TTF font path. `KIWIX_DEBUG=1` for debug logging.

## License

GPL-3.0
