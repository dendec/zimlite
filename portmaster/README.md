## Overview

Ever wanted the whole of Wikipedia in your pocket, with no Wi-Fi, no data plan, and no compromises?
**Zimlite** turns your handheld into a pocket library. It's a fast, lightweight offline reader for **ZIM archives** — the same format used by Kiwix — so you can browse Wikipedia, Wiktionary, Wikibooks, Project Gutenberg, StackExchange dumps, and thousands of other offline archives straight from your device, no internet required.

Perfect for long trips, camping, flights, or just keeping a searchable encyclopedia handy on your favorite retro handheld.

### Features
- Read full ZIM archives completely offline
- Alphabetical article tree for browsing the entire archive's catalogue
- Click-through links between articles with back/forward history
- Built-in downloader to fetch ZIM archives directly from the Kiwix library when online

### How to use:
1. Download ZIM files (e.g. Wikipedia, Wikibooks, Wikivoyage, etc.) for free from **[library.kiwix.org](https://library.kiwix.org)**.
2. Copy the `.zim` files into the port directory:
   `/roms/ports/zimlite/`
3. Restart **Zimlite**. It will automatically detect and load the archive!

## Notes

Thanks to the [Kiwix Project](https://kiwix.org) for their work curating and distributing free offline ZIM content, and to the [openZIM project](https://wiki.openzim.org/wiki/Libzim) for the libzim library used to read it.

Project homepage: [dendec.github.io/zimlite](https://dendec.github.io/zimlite/)

## Controls

| Button | Action |
|--|--| 
| D-Pad Up / Left | Select previous link |
| D-Pad Down / Right | Select next link |
| Left Analog Stick Up / Down | Scroll article |
| Left Analog Stick Left / Right | Scroll a wide table horizontally |
| A | Open selected link |
| B | Go back in history |
| X | Toggle the article tree; in search, switch Shift or focus the input |
| Y (tap) | Go to the archive's main article |
| Y (hold) | Open the application menu |
| L1 / R1 | Page Up / Page Down |
| Menu / Select | Open Settings; Select focuses the input during Search |
| Start | Open Search in a ZIM archive; in Search, switch between input and results |

Tap Y returns to the archive's main article, or to the application menu when no archive is open. Hold Y opens the application menu.

## Compile

```shell
# Cross-build ARM64 binary via Docker
make dist-portmaster
# Output: dist/zimlite.zip
```

## Article renderer

Settings → Article renderer switches between the existing SDL renderer and
Servo. SDL is the default. Servo renders the original HTML and CSS; article
scripts and external network resources are disabled. Menus and Markdown keep
using SDL. If Servo fails or exceeds its memory budget, the article falls back
to SDL with a status message.

The optional package includes the helper, CPU libraries, fonts, licenses and a
private software Mesa runtime. Software rendering is the default while firmware
GLES compatibility with the full SDL app is being validated. Setting
`ZIMLITE_SERVO_CONTEXT=auto` opts into firmware GLES with software fallback when
a context cannot be created. Nothing is installed into the
firmware's library directories. A 1 GB device still needs available memory for
the OS and viewer; large pages may reach the configured budgets.

```shell
make dist-portmaster-servo
# Output: dist/zimlite-servo.zip
```

The build audits runtime symbols against glibc 2.30. Hardware acceptance and
current compatibility limits are recorded in `docs/SERVO-PROTOTYPE-RESULTS.md`.

For memory diagnostics, create `zimlite/servo/profile.enabled` and launch the
port. Each run writes app events, Go memory, process/system memory and kernel
messages into temporary RAM files, then archives them in
`zimlite/profiles/<UTC-date>-<PID>/` after normal exit. Active logs disappear on
reboot; copy them over SSH during investigation. Remove the flag to disable.
See `docs/SERVO-PROFILING.md` for log fields and limits.

When the optional Servo runtime is installed, the launcher stages the Go
executable (~22 MiB) in `/tmp` before SDL startup. `KIWIX_APP_DIR` keeps
configuration, catalogue cache and helper paths in the installation directory.
The temporary executable is removed after exit. This avoids executing the
Go binary from the handheld FUSE/exFAT partition while Servo starts.
