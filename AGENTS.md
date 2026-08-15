When you change Go code, before stopping, run go fmt, go fix, go mod tidy, and golangci-lint, and test what you changed.

When you need context in tests, use t.Context().

Do not use dot imports (`. "maragu.dev/gomponents/html"`) — the linter forbids them. Use qualified imports (`"maragu.dev/gomponents/html"` then `html.P(...)`) instead.

The gomponents `html` package exports a `Nav` function, so avoid naming your own functions `Nav` in packages that import it.

`html.StyleAttr` is deprecated. Use `html.Style` instead.

Constructor functions that perform I/O (e.g. network calls, auth) should accept a `context.Context` parameter. In cobra commands, use `cmd.Context()` as the parent context rather than `context.Background()`.

To run the app: go run . web --addr ":25256"

Use the browser to navigate to http://localhost:25256/ to see the app in action if needed.

## Project specifics

`go run . phonikud "שָׁלוֹם"` runs everything below the diacritizer with no models
loaded, which is the fastest way to check a change. The `phonikud` skill in
`.claude/skills/` covers the rest of the testing story.

Hebrew, niqqud and IPA are all multi-byte in UTF-8. **Iterate runes, never bytes**, anywhere
either is involved. A `for i := 0; i < len(s); i++` over Hebrew or IPA silently corrupts it,
and that is by far the likeliest source of a bug in this codebase.

`pkg/phonikud` is a frozen fork of the Python `phonikud` 0.4.1 finite-state transducer. Its
rules look strange because they encode Hebrew edge cases that a native speaker validated.
Do not "fix" or tidy them; `pkg/phonikud/testdata/corpus.json` will tell you if you did.
There is deliberately no mechanism to sync with upstream.

The two ONNX models are not in the repo. `make models` (or the Dockerfile) downloads them
and verifies their SHA-256. If a checksum ever fails, stop — the voice was chosen by human
listening and a silent substitution would invalidate that.

Do not change the synthesis parameters in `pkg/voice` or the `" ."` tail appended before
synthesis. Both were settled by blind human listening — the voice itself was picked that
way, over seven others, under exactly the parameters now in the file — and there is no
automated substitute for that test.

The voice takes its symbol table and sample rate from the checkpoint's own ONNX metadata,
so this package ships no config file. It also interleaves a blank around every phoneme,
which is what its checkpoint was trained on; other checkpoints are trained without it and
turn into fluent babble at more than twice the right length if given it. Phoneme encoding
does not carry over between voices by assumption.

Always emit `<meta charset="utf-8" />` **and** serve `Content-Type: text/html; charset=utf-8`.
Missing either renders Hebrew as mojibake.
