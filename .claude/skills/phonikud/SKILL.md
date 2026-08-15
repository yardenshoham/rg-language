---
name: phonikud
description: Test and debug the RG pipeline — phonemize vocalized Hebrew, check the RG transform and the three renderings, run the differential corpus, and hear the output. Use whenever changing pkg/phonikud, pkg/rg, pkg/heb, pkg/diacritizer, pkg/voice or pkg/pipeline, or when a Hebrew word comes out wrong.
---

# Testing the RG pipeline

The pipeline splits cleanly in two, and that split is what makes it testable:

```
Hebrew text ──diacritizer──> Hebrew + niqqud ──phonikud──> IPA ──rg──> RG IPA ──voice──> WAV
            └ 300 MB model ┘                 └────── pure string handling ──────┘└ 131 MB model ┘
```

Everything to the right of the niqqud is deterministic string-to-string, so it can
be checked exactly and instantly. Start there.

## The fast loop: `phonikud`

Needs no models and no ONNX Runtime, so it answers in milliseconds. Give it Hebrew
that already has niqqud.

```bash
go run . phonikud "שָׁלוֹם"
```

```
niqqud     שָׁלוֹם
ipa        ʃalˈom
rg ipa     ʃaʁɡalˈoʁɡom
rg         שרגלורגום
rg niqqud  שָׁרְגָלוֹרְגוֹם
rg latin   sha-rga-lo-rgom
```

Useful flags:

- `--json` — one object per line, with the same field names as the corpus, so the
  output can be diffed against `pkg/phonikud/testdata/corpus.jsonl` directly.
- `--stress second|both` — the other stress policies. `first` is the default and
  the one blind listening chose; the others exist because stress may be mildly
  word-dependent.
- `--raw` — skip the niqqud repair and the override lexicon, to see what the
  transducer does with the input exactly as given.

It reads one phrase per line from stdin when given no arguments:

```bash
printf 'גַּנָּן\nנֶחְמָד\n' | go run . phonikud --json
```

## The whole pipeline, including the models

`say` runs everything and optionally writes the audio. It needs the models, so
run `make models onnxruntime` once first.

```bash
make say TEXT='"אני ממש אוהב פיצה"'
go run . say "מה נשמע" --out debug/hello.wav      # with the env vars already set
```

`make run` starts the site on :25256.

## The test suites

```bash
make test        # no models needed; replays the whole corpus below the diacritizer
make test-full   # adds the diacritizer over all 5,012 items, a few minutes
make lint        # go fmt, go fix, go mod tidy, golangci-lint
```

`make test` covers:

- the 8 reference sentences the project is defined by (`pkg/rg`)
- the 7 Hebrew spellings (`pkg/heb`)
- all 5,012 corpus items through phonemize, transform, heb and latin
- the blank-interleaved phoneme ids the voice is fed, and that the checkpoint's
  own symbol table covers every phoneme the corpus can produce (needs the model)

## The differential corpus

`pkg/phonikud/testdata/corpus.jsonl` is 5,012 Hebrew words and sentences run
through the original Python implementation once, recording what every stage
produced. The Go port matches it byte for byte at every stage, including the
diacritizer.

The fork of phonikud is frozen on purpose and there is no upstream sync, so the
corpus stays valid indefinitely. **If a change makes the corpus fail, the change
is wrong** — the rules in `pkg/phonikud` look arbitrary because they encode
Hebrew edge cases a native speaker validated. Do not "fix" them.

To see what changed:

```bash
go test -run TestCorpus ./pkg/phonikud/          # prints the first 10 mismatches per stage
```

Regenerating the corpus needs the Python lab in `debug/lab/`, which may not be
present in a fresh clone. If it is:

```bash
cd debug/lab && ./.venv312/bin/python ../gen_corpus.py /tmp/corpus.json
```

`.venv312` was made by `uv` and has no `pip`; install with
`uv pip install --python .venv312/bin/python <pkg>`.

## When a word comes out wrong

Work out which half is at fault by feeding the niqqud in by hand:

```bash
go run . say "חכמה"                    # what the diacritizer produced
go run . phonikud "חׇכְמָה"              # what it should have produced
```

If the correct niqqud gives the right IPA, the diacritizer is at fault, and the
fix is **an entry in `pkg/pipeline/lexicon.json`** mapping the bare word to its
pinned niqqud. Pinning at the niqqud level keeps the phonemes, the transform and
the Hebrew rendering all on the normal path. Add the entry, then re-run
`make test`.

If the correct niqqud still gives the wrong IPA, the transducer is at fault — and
the corpus test will almost certainly already be failing, which tells you where.

## Audio

There is **no automated audio-quality test, and none should be built.** One was:
the transcriber had a 25–30% phoneme error rate on ordinary Hebrew and
false-flagged 14 of the 15 clips a human had passed. Do not tune anything against
a transcriber.

The synthesis parameters in `pkg/voice` and the `" ."` tail appended before
synthesis were both settled by blind human listening — the voice itself was
chosen that way, over seven others, under exactly these parameters. Do not
change either without a new listening round.

Known voice defects, none of which are the transform's fault: word-final
ד/ת/ק/ג/ב can be soft, vowel colour sometimes drifts on the copied vowel, and
`רג` is weak in a few words. All are milder in sentences than in single words.
The first of those was the previous voice's worst failing and drove the switch to
Matcha, so it is improved rather than gone.

## Traps

- **Iterate runes, never bytes.** `ʁ ɡ ˈ` and every Hebrew letter and mark are
  multi-byte. This is the likeliest source of silent corruption here.
- Combining marks are written as `\uXXXX` escapes in Go source in these packages.
  As literal characters they attach to whatever precedes them and become
  impossible to edit reliably.
- Never overwrite `debug/lab/out/results-*.json` — those are irreplaceable human
  listening labels.

## Related

For anything that has to happen in a browser — layout, niqqud placement, RTL,
audio playback, the htmx wiring — use the `playwright` skill instead. It can also
take a screenshot you can look at.
