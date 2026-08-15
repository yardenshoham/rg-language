# rg-language

שפת הריש גימל — type a Hebrew sentence, get it back in RG, and hear it spoken.

The rule is one line: **after every vowel, insert רג and a copy of that vowel.**
So שלום becomes שרגלורגום, and מה נשמע becomes מרגה נרגשמרגע.

Everything runs locally. Two ONNX models do the hard parts — one adds niqqud to
plain Hebrew, one turns phonemes into speech — and the RG rule itself is about
eighty lines of string handling.

## How it works

```
Hebrew text
  -> diacritizer      add niqqud, stress and vocal-shva marks (ONNX BERT)
  -> normalize        repair a doubled-vowel artifact
  -> lexicon          pin hand-corrected words
  -> phonemize        -> IPA with stress, e.g. ʃalˈom
  -> transform        -> RG IPA,           e.g. ʃaʁɡalˈoʁɡom
  -> voice            -> 22.05 kHz mono WAV (ONNX Matcha-TTS)
```

The transform works on IPA rather than on Hebrew letters, which is what makes it
small: in IPA the rule really is "insert `ʁɡ` and a vowel copy", while in Hebrew
orthography the same rule needs matres lectionis, final letter forms, dagesh and
silent א/ה/ע as special cases. It also needs no syllabification at all — moving a
consonant across a syllable boundary does not change where it lands relative to
the inserted material, so only the vowel positions matter.

Only the diacritizer can surprise you. Everything below it is deterministic
string-to-string code, pinned byte-for-byte by a 5,012-item corpus in
`pkg/phonikud/testdata/`.

## Quick start

The models are not in the repo — they are 439 MB — so fetch them once:

```bash
make models
make onnxruntime
```

Then:

```bash
export ONNXRUNTIME_LIB=$PWD/debug/onnxruntime/lib/libonnxruntime.so
export RG_MODELS_DIR=$PWD/debug/models
go run . web --addr :25256
```

Open <http://localhost:25256>.

### Without the models

Everything below the diacritizer is pure string handling, so `phonikud` runs it
on Hebrew that already has niqqud — no models, no ONNX Runtime, no wait. It is
the fast way to check a rule:

```bash
go run . phonikud "שָׁלוֹם"
printf 'גַּנָּן\nנֶחְמָד\n' | go run . phonikud --json
```

`--json` emits the same field names as the differential corpus, so its output can
be diffed against `pkg/phonikud/testdata/corpus.jsonl` line by line.

### With the models

There is a one-shot command, handy for checking output by ear:

```bash
go run . say "אני ממש אוהב פיצה" --out debug/pizza.wav
```

```
input      אני ממש אוהב פיצה
niqqud     אֲנִי מַמָּשׁ אוֹהֵב פִּיצָה
rg         ארגנירגי מרגמרגש אורגוהרגב פירגיצרגה
rg niqqud  אֲרְגַנִירְגִי מַמָּרְגָשׁ אוֹרְגוֹהֵרְגֵב פִּירְגִיצָרְגָה
rg latin   a-rga-ni-rgi ma-rga-ma-rgash o-rgo-he-rgev pi-rgi-tsa-rga
```

### Docker

The image downloads ONNX Runtime and both checkpoints, verifying their SHA-256:

```bash
docker build -t rg-language .
docker run -p 25256:25256 rg-language
```

## Configuration

| Environment variable | Flag               | Default                             | Description                                    |
| -------------------- | ------------------ | ----------------------------------- | ---------------------------------------------- |
| `RG_ADDR`, `PORT`    | `--addr`           | `:25256`                             | HTTP listen address                            |
| `RG_MODELS_DIR`      | `--models`         | `/models`                           | Directory holding the two `.onnx` files        |
| `RG_AUDIO_CACHE_MB`  | `--audio-cache-mb` | `256`                               | Synthesized audio kept in memory               |
| `ONNXRUNTIME_LIB`    |                    | `/usr/local/lib/libonnxruntime.so`  | ONNX Runtime shared library                    |
| `RG_POSTHOG_KEY`     | `--posthog-key`    |                                     | PostHog project API key; enables analytics     |
| `RG_POSTHOG_HOST`    | `--posthog-host`   | `https://eu.i.posthog.com`          | PostHog ingestion host                         |
| `RG_POSTHOG_UI_HOST` | `--posthog-ui-host`|                                     | PostHog dashboard host, when ingestion is proxied |

## Analytics

Off unless `RG_POSTHOG_KEY` (or `--posthog-key`) is set — with no key the pages
carry no tracking script at all. `RG_POSTHOG_HOST` points ingestion somewhere
else, typically a reverse proxy; when it does, `RG_POSTHOG_UI_HOST` is what
makes links in the captured events point back at PostHog itself.

## Tests

```bash
go test ./...
```

That replays the whole differential corpus through the phonemizer, the transform
and both renderings.

The diacritizer test needs the models, and walks a 200-item sample by default:

```bash
RG_MODELS_DIR=$PWD/debug/models RG_FULL_CORPUS=1 go test ./pkg/pipeline/
```

Browser tests live in `e2e/` and drive the real site with Playwright, including
the eight reference sentences end to end:

```bash
cd e2e && npm install && npx playwright install chromium   # once
make e2e
```

`node e2e/shot.mjs --text "מה נשמע" --theme dark` screenshots a page.

There is deliberately **no automated audio-quality test**. One was built and
failed its validation gate: the transcriber had a 25–30% phoneme error rate on
ordinary Hebrew and false-flagged 14 of the 15 clips a human had passed. Any
change that touches synthesis needs a person to listen.

## Known rough edges

These come from the models, not from the transform — across 34 graded items not
one failure was ever attributable to the RG rule itself.

- Word-final ד/ת/ק/ג/ב can still be soft. This was the previous voice's worst
  defect — eight voices were measured, none released them, and nineteen
  input-level workarounds all failed. The current voice was picked in a blind
  round partly on the sentence built to expose it, so it is better, not solved.
- Vowel colour sometimes drifts on the copied vowel.
- Isolated homographs (שוק) and kamatz katan (חכמה) need lexicon entries; in a
  full sentence the diacritizer usually gets them right from context.

All of it is milder in sentences than in single words, which is why the site
nudges you towards sentences.

New diacritizer mistakes are fixed by adding an entry to the `lexicon` map in
`pkg/pipeline/niqqud.go` and redeploying. Pinning at the niqqud level keeps the phonemes, the transform
and the Hebrew rendering all on the normal path.

## Disclaimer

This is a hobby project. It was vibe-coded, and none of the code has been
reviewed. Treat it accordingly — do not use it for anything that matters.

## Licences

The code is Apache 2.0 — see [`LICENSE`](LICENSE). Copyright Yarden Shoham.
One exception: `pkg/phonikud` is a fork of the Python `phonikud` transducer, so
that directory carries its upstream terms too.

The models are separate. The diacritizer is CC BY 4.0, and the voice checkpoint
is MIT.
