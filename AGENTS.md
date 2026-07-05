# AGENTS.md — Kiwix SDL Codebase Map

## Identity

**kiwix-sdl** — offline ZIM/markdown viewer for game consoles (PortMaster/TrimUI) and desktop. Go + C/C++ (cgo). SDL2 graphics, libzim for ZIM archives, LunaSVG for emoji/SVG.

## Entry Point

`cmd/kiwix-sdl/main.go` — starts SDL2, loads config, creates renderer, sets up App, runs event loop.

## Architecture Flow

```
main.go
  ├── config.Load()                   → internal/config/config.go
  ├── renderer.New()                  → internal/renderer/renderer.go
  │     ├── sdl.Init + ttf.Init
  │     ├── gamepad init
  │     └── loadFonts()               → internal/renderer/fonts.go
  ├── ui.New(r, r, r, navigator)      → internal/ui/ui.go
  │     ├── DocumentLoader             → internal/ui/loader.go
  │     └── InputController            → internal/ui/input.go
  ├── app.OpenFile(path)              → loader → menu/parser/storage
  └── app.Run()                       → event loop (SDL poll + render)
```

## Package Map

### `cmd/kiwix-sdl/main.go`
- Entry point. Parses CLI arg as file/virtual path. Sets logger. Locks OS thread (SDL requirement).
- Calls `findFont()` for `$KIWIX_FONT`.
- Falls back to menu on file open failure.

### `internal/config/config.go`
- Singleton `Config` (Language, Theme, FontSize).
- `Load()` / `Save()` — JSON file next to binary.
- `Get()` / `Set()` — RWMutex protected.
- `Default()` — dark theme, font 16.

### `internal/document/`
**Pure data — never depends on SDL or parsers.**

- **Block types:** Heading, Paragraph, List, CodeBlock, Table, ThematicBreak, Blockquote, Link, Image
- **Inline types:** Text, LinkInline, ImageInline, Emphasis, Strong, Code, SoftBreak, HardBreak
- **Visitor pattern:** `BlockVisitor` interface + `VisitBlocks()`
- **Word conversion:** `InlineWordVisitor` flattens inlines to `[]Word` for word-wrap layout
- **Emoji detection:** `emoji.go` — Unicode range tables, ZWJ sequence parsing, flag pairs, keycaps. `tokenizeText()` splits text into space/word/emoji tokens.

### `internal/markdown/parser.go`
- goldmark AST → `document.Document`.
- `goldmark.New()` with `extension.Table`.
- Handles: headings, paragraphs, fenced code blocks, thematic breaks, blockquotes, tables (gfm), lists (nested → flat), inlines (bold/italic/code/links/images/raw HTML).

### `internal/html/parser.go`
- HTML → Markdown → Document.
- Uses `html-to-markdown/v2` with base + commonmark + table + math plugins.
- `MathPlugin()` — handles Wikipedia `<span class="mwe-math-element">` → `![alt](src)` image fallback.
- `preprocessTables()` — fixes mixed th/td rows.

### `internal/zim/`
**cgo bridge to libzim C++ library.**

- `bridge.h` / `bridge.cpp` — extern "C" wrappers: open, close, get entry, get item, list articles, free.
- `zim.go` — Go `Reader` struct wrapping C handle. Methods:
  - `Open(path)` — opens archive, resolves main page redirect
  - `ListArticles()` — `iterByTitle()` → flat array of title/path
  - `ResolveArticle(rawURL, referrer)` — tries multiple path candidates (exact, cleaned, relative to referrer, prefixed with root)
  - `ResolveResource(path)` — tries `I/`, `-/`, `images/` namespace prefixes
  - `GetResource()` — raw bytes + mimetype
- `dummy.go` — stub for `!cgo` build tag.

### `internal/renderer/`
**SDL2 rendering engine. Central component.**

- `renderer.go` — `Renderer` struct: owns SDL window/renderer, fonts, texture cache, layout cache.
  - `New()` — SDL init, window creation, font loading.
  - `SetDocument()` — clears scroll, extracts title, triggers relayout.
  - `ToggleTheme()` — switches Light/Dark, relayouts.
  - `Zoom(delta)` — reinitializes fonts at new size.
  - Link/scroll APIs delegate to layout.
  - `FindAnchorY()` — searches headings and footnote backlinks.
- `layout.go` — `layoutState` implements `BlockVisitor`. Lays out blocks into `PageLayout` (lines, links, code ranges, blockquotes, images, tables). Word-wrap via `InlineWordVisitor`.
- `draw.go` — `Render()` draws: images → blockquotes → code bg → tables → link highlight → text lines → scrollbar → status bar. Texture caching for text+emoji.
- `theme.go` — Light/Dark color palettes.
- `fonts.go` — Embedded `unifont.otf` (fallback). `loadFonts()` loads 8 font sizes (body, h1–h6, mono). `measureText()` handles emoji sizing.
- `fonts_default.go` — Font loading wrappers for SDL_ttf.
- `emoji.go` — Embedded `emoji.zip` (Twemoji SVGs). `getEmojiSVG(hex)` opens from in-memory zip.
- `image_manager.go` — Loads PNG/JPEG/GIF/SVG/webp. `ImageManager` caches textures. GIF animation with frame timing.
  - `GetDimensions(url)` — loads & caches image dimensions.
  - `GetTexture(url)` — returns SDL texture + animated flag.

### `internal/ui/`
**Application logic.**

- `ui.go` — `App` struct. Modes: `modeDoc` / `modeTree`.
  - `Run()` — event loop: WaitEvent → PollEvent → ProcessEvent → Render(). Background ticker for animations.
  - `goBack()` — handles tree exit, history back, ZIM tree fallback, menu fallback.
  - `enterTreeMode()` / `exitTreeMode()` — Radix tree for ZIM articles.
  - `HandleSettingsAction()` — parses `virtual:settings?theme=&fontsize=&lang=` URL.
- `interfaces.go` — DI interfaces: `DocViewer`, `LinkBrowser`, `Scroller`, `DocNavigator`, `ZimReader`.
- `input.go` — `InputController` processes SDL events: keyboard, gamepad (ControllerAxis/Button), mouse (wheel, click), window resize.
  - `processDocKey()` / `processTreeKey()` — mode-specific key dispatch.
  - `executeGamepadAction()` — maps `Action` to function calls.
  - `handleTreeSelection()` — opens article or expands tree node.
- `loader.go` — `DocumentLoader`: caching, ZIM lifecycle, virtual pages, resource loading.
  - `OpenFile(path)` — opens .md, .html, .zim, or `virtual:*` pages.
  - `NavigateLink(url)` — handles external URLs, virtual settings/delete, anchors, ZIM article resolution.
  - `startDownload()` — background download with progress via `SetStatusOverride()`.
  - `checkInternetAsync()` — background Kiwix library ping.
- `library.go` — Kiwix online library browser: OPDS/Atom feed parser. Three pages: languages → categories → entries (paginated). Download triggers via `virtual:library/download`.
- `gamepad.go` — `GamepadState` with `TriggerDebouncer`. Translates SDL controller events to `Action` enum.
  - Actions: OpenEnter, Back, ScrollUp/Down, PageUp/Down, ToggleTree, GoHome, Quit, ZoomIn/Out, SelectPrevLink, SelectNextLink, ToggleTheme.

### `internal/menu/`
**Virtual page generators (markdown template → Document).**

- `menu.go` — Templates embedded via `//go:embed`.
  - `FileSelector()` — scans CWD for .zim/.md/.html files, renders menu template.
  - `HelpPage(hasGamepad)` — keyboard or gamepad help.
  - `SettingsPage()` — theme/language/font size controls.
  - `CheckInternet()` — pings Kiwix library catalog.

### `internal/navigation/navigation.go`
- `SimpleNavigator` — stack-based history with `HistoryItem{ID, ViewState}`.
- `Open(id)` — truncates forward history, pushes new.
- `Back()` — pops to previous.

### `internal/storage/storage.go`
- `OpenFile(path)` — .md → markdown.Parse, .html → html.Parse.
- `OpenZIM(path)` — opens archive, parses main page HTML.
- `FormatSize()` — human-readable byte sizes.
- `Download(url, filename, onProgress)` — HTTP download with progress callback.

### `internal/trie/`
**Radix tree for ZIM article tree browser.**

- `radix.go` — `RadixNode` with lazy expansion. First level (A-Z, А-Я, 0-9) built eagerly from article list. `Expand()` builds children on demand. Auto-collapses single-child chains.
  - `NewTree(articles)` — groups by first rune.
  - `Label()` / `Suffix()` / `FullPath()` — display helpers.
- `nav.go` — `NavState` tracks cursor. `MoveUp/Down/To`, `ActionRight/Left` (expand/collapse/parent).
  - `VisibleNodes()` → `[]VisLine` with Unicode tree connectors (├── └── │).

### `internal/svg/`
**Embedded LunaSVG C++ library (20+ .cpp/.h files).**

- `wrapper.go` — `Render(data)` and `RenderToSize(data, w, h)` — cgo calls to lunasvg for SVG → RGBA.

## Key Data Structures

```
document.Document { Blocks []Block }
  Block → Heading | Paragraph | List | CodeBlock | Table | ThematicBreak | Blockquote | Link | Image
  Inline → Text | LinkInline | ImageInline | Emphasis | Strong | Code | SoftBreak | HardBreak
  Word { Text, IsSpace, IsEmoji, IsImage, PixW, PixH, LinkID, IsBold, IsItalic, IsCode, IsHardBreak }

renderer.PageLayout { lines, links, codeRanges, codeSpans, blockquotes, imageEntries, tables, totalHeight, contentWidth }
  lineEntry { text, fontIdx, color, x, y, w, h, isBold, isItalic, isCode, isCursor, isEmoji, emojiHex }
  linkEntry { rects []sdl.Rect, url string }

trie.RadixNode { prefix, leaf, children, parent, expanded, articles }
trie.NavState { Root, Cursor }
trie.VisLine { TreePrefix, Label, Suffix, IsLeaf, IsExpanded, IsCursor }
```

## Build System

- `Makefile` — `make build`, `make test`, `make lint`, `make clean`, `make run`
- Перед коммитом проверяй код через `make test && make lint`
- Cross-build targets: `build-linux-amd64`, `build-linux-arm64`, `build-linux-armv8`, `build-windows-amd64`
- Docker: `Dockerfile.arm64` (cross-build for TrimUI), `Dockerfile.windows`
- PortMaster packaging: `dist-portmaster` creates `dist/kiwix-sdl.zip`
- CGO required. libzim auto-downloaded or cross-built from source.

## Known Issues (see ISSUES.md)

- Config package still uses global singleton internally (partially addressed via Provider DI)
- No errcheck (disabled in .golangci.yml)
- HTTP clients not unified
- Library atom XML has typo in namespace field tag

## Test Files

- `internal/ui/interfaces_test.go` — interface compliance check
- `internal/navigation/navigation_test.go` — SimpleNavigator tests
- `internal/zim/parser_test.go`
- `internal/html/parser_test.go`
- `internal/markdown/parser_test.go`
- `internal/document/emoji_test.go`
- `internal/renderer/emoji_test.go` (if exists, not confirmed)
- `internal/trie/radix_test.go`
- `internal/svg/wrapper_test.go`
