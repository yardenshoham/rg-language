---
name: playwright
description: Drive the site in a real browser — run the end-to-end suite, take a screenshot and look at it, or debug a page interactively. Use whenever changing internal/web, the CSS, the fonts or the htmx wiring, when asked whether the site works or looks right, or when a browser-only behaviour (RTL, niqqud placement, audio playback, live typing) is in question.
---

# Driving the site with Playwright

Everything lives in `e2e/`. It is development-only — the site itself ships no
JavaScript build and no node_modules.

## Setup, once

```bash
cd e2e && npm install && npx playwright install chromium
```

The tests also need the models, because the site does not start without them:

```bash
make models onnxruntime
```

## Run the suite

```bash
cd e2e && npx playwright test
```

The config starts the server itself (`go run . web --addr :25256`) and waits on
`/health`, which does not answer until both ONNX models are loaded — so waiting on
the port is waiting on readiness. An already-running server on that port is
reused, which makes the loop fast while iterating.

Useful invocations:

```bash
npx playwright test -g "highlights"      # one test by name
npx playwright test --headed             # watch it happen
npx playwright test --debug              # step through with the inspector
npx playwright test --ui                 # the time-travel UI
npm run report                           # open the HTML report after a failure
```

Failures leave a screenshot and a trace under `tmp/e2e-results/`. Open a trace
with `npx playwright show-trace <path>`.

## Look at the page

This is the part worth reaching for often: take a screenshot, then **Read the PNG
file** — you can see it, so you can judge layout, niqqud placement and colour
directly rather than inferring them from the DOM.

```bash
cd e2e
node shot.mjs                                        # home page -> tmp/shot.png
node shot.mjs --text "אני ממש אוהב פיצה" --out tmp/home.png
node shot.mjs --text "מה נשמע" --theme dark --out tmp/dark.png
node shot.mjs /about --width 420 --out tmp/about-mobile.png
```

Flags: `--text` (fills the box and renders the result), `--theme light|dark`,
`--width`, `--out`, `--base` (default `http://127.0.0.1:25256`). The server must
already be running — `make run` in another terminal.

The script waits for `document.fonts.ready` before shooting. Without that the
screenshot can catch the fallback font, which puts the niqqud in the wrong place
and makes a fine page look broken.

## What the suite covers

- the 8 reference sentences the project is defined by, end to end in a browser
- RTL, `lang="he"`, and the UTF-8 content type
- both weights of the bundled Hebrew font actually loading
- the page working with **JavaScript disabled** (the form is a plain GET; htmx is
  an enhancement, not a requirement)
- live updating as you type, without a navigation
- the inserted רג being highlighted and nothing else
- audio: not fetched until play is pressed, correct content type and immutable
  caching, a real 22.05 kHz mono WAV that the browser can decode, 404 on an
  unknown hash
- HTML in the input being escaped

## Writing more tests

Assert Hebrew with the `expectHebrew` helper in `site.spec.js`, not with
`toHaveText`. The diacritizer writes the shin dot before the vowel, which is not
canonical mark order even though it renders identically; the helper compares
NFC-normalized text so an assertion is about the letters, not about the order the
marks happen to be stacked in.

Note that the inserted run is not always just `רג` — the o and u copies carry a
vav, so `שלום` ends `...רגו`.

## Traps

- Never `pkill -f` a server by pattern; the pattern matches the shell running it.
  Find the pid with `ss -ltnp | grep 25256` and kill that.
- Headless Chrome has no audio device, so `currentTime` does not advance. Assert
  on `duration` and `readyState` instead — decoding is the part that proves the
  server built a playable WAV.
