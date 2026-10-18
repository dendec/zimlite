# Offline text input runtime

Three small adapters with separate responsibilities:

- `main.cpp`: one m17n input method per process, UTF-8 composition and candidates.
- `search.cpp`: one libzim/Xapian query per process, ten title/path results, no snippets.
- `text.cpp`: optional SDL keyboard shaping backend (HarfBuzz, FreeType, FriBidi).

`make input-runtime-arm64` and `make input-runtime-amd64` build the offline
runtime in Docker. Keep the resulting directory next to the application as
`input/`, or set `KIWIX_INPUT_RUNTIME_DIR`.

m17n worker arguments: language, method, hex UTF-8 initial text, runtime path.
Each request is `operation<TAB>hex UTF-8 value<LF>`. Operations: `key`, `text`,
`commit`, `cancel`, `backspace`, `replace`. `text` is already committed OS text;
`key` is a symbolic key for the on-screen IME. Space is a literal ASCII space.
State replies contain committed text, preedit, selected candidate, candidate
count, candidates, and optional key labels; all strings are hex encoded.
Labels are sent once. Every command gets a complete state reply in order.
The Go client bounds queues and messages, matches replies before switching
methods, and deletes committed graphemes with Unicode 17 rules.

Workers are killed on cancellation, timeout or low available system RAM.
Closing/switching methods drops all m17n global caches by ending the process.
Chinese uses `zh-py`; `zh-pinyin` produces Latin tone marks. Japanese uses
Anthy, its m17n module and bundled dictionaries with relocated paths.
All method data ships offline; none is downloaded while typing.

PC search links the official libzim 9.7 binary, requiring glibc 2.34 and
libstdc++ from GCC 12 or newer. ARM64 libzim/Xapian is compiled using GCC 10
for handheld firmware compatibility. A Windows native runtime is not built.
