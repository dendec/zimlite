# Code review: Servo integration and handheld controls

Reviewed branch: `feature/servo-article-renderer`, through `dfb9140`, plus the
corrections in this working tree. Review covered application code introduced by
the branch and its interaction with the existing loader, history, SDL rendering,
archive index, input, and resource lifecycle. The upstream Servo source and the
two untracked keyboard repositories were outside this review.

## Findings corrected

| Priority | Finding | Correction |
| --- | --- | --- |
| P1 | Held input accelerated exponentially without a cap. A long hold could overflow a scroll delta; tree repeat could reach a zero interval and divide by zero. A stalled frame could process excessive navigation work. | Cap acceleration and repeat frequency, bound catch-up to 100 ms, and keep one tree hold that resets when direction or mode changes. |
| P1 | Background downloads wrote the status string while SDL read it without synchronization. | Protect status access with an RWMutex; rendering reads a snapshot through the accessor. A concurrent regression test runs with the race detector. |
| P2 | Stick polling used raw joystick axis numbers, bypassing the SDL controller mapping. | Poll the logical axes through `GameController.Axis`. |
| P2 | Axis debounce ignored a direct reversal until the stick passed through neutral. | Track the active direction and fire when it reverses. |
| P2 | Failed texture replacement after a Servo resize retained the pointer to an already destroyed SDL texture. | Clear the pointer immediately after destruction. |
| P2 | Image dimensions survived cache clearing across documents with identical relative image names, retaining metadata and potentially using incorrect sizes. | Clear dimensions with the image cache; test the same URL across different image sizes. |
| P2 | Table scroll capability could describe an earlier selected link while selection commands were still in flight. | Reset capability on selection and accept capability updates only for the matching selected link. Clear stale view errors when stopping the article. |
| P2 | Restoring a cached virtual/Markdown page could retain the previous archive reader and tree. History restoration applied scroll and selection even after a failed load. | Consolidate restoration, close archive state for ordinary cached documents, and apply saved view state only after a successful load. |
| P2 | HTML table wrapping claimed to preserve markup, but `Tokenizer.TagName` lowercased its shared raw buffer. | Preserve raw tag bytes before inspecting the tag name; regression coverage includes uppercase and nested tables. |
| P2 | `make test` included temporary Go diagnostic programs from the ignored `dist` directory. Lint/vet did not consistently receive native build flags. | Scope checks to `cmd` and `internal`, and supply the same cgo flags used for the application build. |

## Module boundaries

| Module | Responsibility | Boundary |
| --- | --- | --- |
| `document` | Document model and presentation contracts (`TreeItem`, `ResourceLoader`, `ViewState`). | No SDL, archive, or engine implementation dependency. |
| `article` | Bounded resource sources, archive adapter, loopback resource server, static HTML table containment. | No SDL dependency. The server borrows the source and waits for active reads before it can be closed. |
| `servo` | Helper process, binary/JSON protocol, bounded queues, memory watchdog. | No SDL dependency. A session owns the helper, server, and source, including startup failures. |
| `renderer` | SDL window, native document rendering, textures, presentation of Servo frames. | Implements the UI's view capabilities; process management stays in `servo`. |
| `ui` | Input, document transitions, settings, fallback, tree and history coordination. | Uses viewer interfaces and shared model types, without importing the concrete renderer package. |
| `navigation` | History and saved view state. | One `Navigator` contract, reused directly by UI. |
| `trie` | Lazy tree navigation over `TitleIndex`. | Depends on the title-index contract and document data, without knowing libzim. |
| `zim` | Native archive and title-index access. | Does not know UI, SDL, or helper process behavior. |
| `native/servo-host` | WebView operations and headless rendering. | Receives semantic commands through bounded IPC; archive resource access stays in Go. |

The unnecessary UI dependency on `renderer` was removed by relocating shared
types into `document`. `ZimReader` now explicitly includes `TitleIndex` and
`ResolveArticlePath`, eliminating silent runtime capability checks and the
fallback that materialized a complete article list. Unreachable duplicate ZIM
loading code and the redundant UI history interface were removed.

DRY: history restoration and hold acceleration now have single implementations;
UI reuses `navigation.Navigator` and `trie.TitleIndex`.

KISS: fixes use existing modules and small helpers. No additional framework,
general-purpose cache abstraction, or new engine hierarchy was introduced.

SOLID: dependencies remain directed through explicit contracts; shared view data
contains no SDL implementation types. Viewer operations retain separate
document, input, status, link, and scrolling interfaces, with optional article
and table capabilities. The concrete renderer remains a facade over the SDL
window and both article presentation paths.

## Validation

- `make test`: all application and internal packages pass.
- `make lint`: zero issues.
- Go race detector on `renderer`, `ui`, `article`, `servo`, and `trie`: passes.
- Rust formatting check on all three helper source files: passes.
- `git diff --check`: passes.
- Added regression coverage for prolonged input, direct stick reversal,
  concurrent status access, image metadata invalidation, preservation of HTML,
  helper startup/protocol failures and source ownership, and bounded navigation
  to the first, middle, and last titles of a generated 10-million-article index.

## Limits of the conclusion

These checks support the corrected module contracts and the tested behavior.
They do not verify a new handheld build, controller mappings on every device,
or native GPU error paths. Rust compilation and device deployment were not run
as part of this review.

The subsequent handheld memory hardening pass adds adaptive physical-memory
budgets, earlier cancellation, bounded simple-image decoding/GIF storage,
asynchronous source cleanup, and fallback from retained HTML. ARM64 compilation
and regression/race checks pass; crash snapshots are now saved during execution.
See `SERVO-PROFILING.md` for the policy and diagnostics.

Memory protection remains sampled rather than a hard cap for the whole
application. Native libzim decompression and uninterruptible kernel/storage
waits remain outside the watchdog's control. Source cleanup waits for an active
native read asynchronously to preserve safety without blocking the SDL thread.
This review, written on 2026-10-11, did not establish a strict 1 GB resource
guarantee or prove the reported handheld hang resolved. Subsequent fixes have
resolved the handheld hangs, as confirmed by the user on 2026-10-18; the older
resource-limit caveat still applies.

The global configuration provider and library cache remain architectural debt.
Their existing interfaces permit incremental isolation. Neither requires
rewriting the renderer or introducing more abstractions for the corrected flows.
