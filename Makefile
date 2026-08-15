# Local development. The container builds everything itself; see Dockerfile.
ORT_VERSION ?= 1.29.0
VOICE_REVISION ?= dcca83dc0911c898fbe4bba464fa450a98c4e7a0
DIACRITIZER_REVISION ?= b806189fe1fc0085b1012b7560ffb5e8ecfd72a2

DEBUG := debug
MODELS := $(DEBUG)/models
ORT := $(DEBUG)/onnxruntime

export ONNXRUNTIME_LIB = $(CURDIR)/$(ORT)/lib/libonnxruntime.so
export RG_MODELS_DIR = $(CURDIR)/$(MODELS)

.PHONY: models onnxruntime run say test test-full e2e shot lint clean

## Download the two checkpoints and verify them. A mismatch is fatal: a substituted
## checkpoint would invalidate the blind listening test this voice won.
models: $(MODELS)/matcha-he-en.onnx $(MODELS)/phonikud-1.0.onnx

$(MODELS)/matcha-he-en.onnx:
	@mkdir -p $(MODELS)
	curl -fsSL -o $@ "https://huggingface.co/thewh1teagle/matcha-tts/resolve/$(VOICE_REVISION)/matcha-he-en.onnx"
	echo "2489ccaf7a2a8cba57011b56f7479a407ad6c21e7a93eddcf62e4788f5eeae4b  $@" | sha256sum -c -

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

## The fast suite: no models, still the whole corpus below the diacritizer. Exports
## cleared or the model tests find the checkpoints (10x slower); -race is what lets
## the cache's concurrency test fail at all.
test:
	RG_MODELS_DIR= ONNXRUNTIME_LIB= go test -race ./...

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
