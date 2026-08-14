# Local development. The container builds everything itself; see Dockerfile.
ORT_VERSION ?= 1.29.0
VOICE_REVISION ?= ff64ba389a3833c49a85c58fc51a7060a6984b87
DIACRITIZER_REVISION ?= b806189fe1fc0085b1012b7560ffb5e8ecfd72a2

DEBUG := debug
MODELS := $(DEBUG)/models
ORT := $(DEBUG)/onnxruntime

export ONNXRUNTIME_LIB = $(CURDIR)/$(ORT)/lib/libonnxruntime.so
export RG_MODELS_DIR = $(CURDIR)/$(MODELS)

.PHONY: models onnxruntime run say test test-full e2e shot lint tidy clean

## Download the two checkpoints and verify them. A mismatch is fatal: the voice
## was chosen by human listening, so a substituted checkpoint would invalidate
## every verdict behind this project.
models: $(MODELS)/shaul_whisper_heb_ipa1.onnx $(MODELS)/phonikud-1.0.onnx

$(MODELS)/shaul_whisper_heb_ipa1.onnx:
	@mkdir -p $(MODELS)
	curl -fsSL -o $@ "https://huggingface.co/Phonikud/phonikud-tts-checkpoints/resolve/$(VOICE_REVISION)/shaul_whisper_heb_ipa1.onnx"
	echo "592363804a09e3d70b05bd86b366e12e0a12c4a4d0100997cf8f0832132c55e7  $@" | sha256sum -c -

$(MODELS)/phonikud-1.0.onnx:
	@mkdir -p $(MODELS)
	curl -fsSL -o $@ "https://huggingface.co/Phonikud/phonikud-onnx/resolve/$(DIACRITIZER_REVISION)/phonikud-1.0.onnx"
	echo "c1fa2624b1e8202a0c0a23259b560b0c41ad92a3a6750bd0e322ce5a2b1acdb6  $@" | sha256sum -c -

## Unpack the ONNX Runtime shared library. It is glibc-based, so this will not
## work on musl.
onnxruntime: $(ORT)/lib/libonnxruntime.so

$(ORT)/lib/libonnxruntime.so:
	@mkdir -p $(ORT)
	curl -fsSL "https://github.com/microsoft/onnxruntime/releases/download/v$(ORT_VERSION)/onnxruntime-linux-x64-$(ORT_VERSION).tgz" \
		| tar -xz -C $(ORT) --strip-components=1 "onnxruntime-linux-x64-$(ORT_VERSION)/lib"

run: models onnxruntime
	go run . web --addr :25256

say: models onnxruntime
	go run . say $(TEXT)

## The fast suite: no models needed, and it still replays the whole corpus
## through everything below the diacritizer.
test:
	go test ./...

## Adds the diacritizer, over all 5,012 corpus items. Takes a few minutes.
test-full: models onnxruntime
	RG_FULL_CORPUS=1 go test -timeout 60m ./...

lint:
	go fmt ./...
	go fix ./...
	go mod tidy
	golangci-lint run

clean:
	rm -rf $(DEBUG)/models $(DEBUG)/onnxruntime

## Browser tests. Needs `cd e2e && npm install && npx playwright install chromium`
## once, plus the models. The config starts the server itself.
e2e: models onnxruntime
	cd e2e && npx playwright test

## Screenshot a page so it can be looked at. The server must be up.
shot:
	cd e2e && node shot.mjs $(ARGS)
