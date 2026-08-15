When you change Go code, before stopping: `go fmt`, `go fix`, `go mod tidy`, `golangci-lint`,
and test what you changed. In tests use `t.Context()`; in cobra commands use `cmd.Context()`.

`go run . web --addr ":25256"` serves the site. `go run . phonikud "שָׁלוֹם"` runs everything
below the diacritizer with no models loaded, which is the fastest way to check a change. The
`phonikud` and `playwright` skills cover the rest of the testing story.

## Two ways to break Hebrew silently

**Iterate runes, never bytes.** Hebrew, niqqud and IPA are all multi-byte in UTF-8, so a
`for i := 0; i < len(s); i++` over any of them corrupts it without any error. This is by far
the likeliest source of a bug in this codebase.

**Emit `<meta charset="utf-8" />` *and* serve `Content-Type: text/html; charset=utf-8`.**
Missing either renders Hebrew as mojibake.

## Things a human settled that no test will re-check

Leave these alone unless asked directly, and do not tidy them into shape:

- **`pkg/phonikud`** — a frozen fork of the Python `phonikud` 0.4.1 transducer. The rules look
  strange because they encode Hebrew edge cases a native speaker validated. There is
  deliberately no mechanism to sync with upstream; `testdata/corpus.json` catches some
  regressions, not all.
- **`pkg/voice`** — the synthesis parameters, the `" ."` tail appended before synthesis, and
  the blank interleaved around every phoneme. This voice won a blind listening test over seven
  others under exactly these settings, and it takes its symbol table and sample rate from its
  own checkpoint's ONNX metadata rather than a config file. Assume none of it carries over to
  another voice.
- **The model checksums** — the two ONNX models are not in the repo; `make models` (or the
  Dockerfile) downloads them and verifies SHA-256. If one ever fails, stop: a silent
  substitution invalidates the listening test above.
