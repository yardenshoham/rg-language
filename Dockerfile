# syntax=docker/dockerfile:1
FROM golang:1.27-bookworm AS builder

# BuildKit fetches these and fails on a digest mismatch. The pins are deliberate: a
# silently substituted checkpoint would invalidate the blind listening test the voice
# won. Above the source copy, so a code change does not re-download ~440 MB.
ARG ORT_VERSION=1.29.0
ARG VOICE_REVISION=dcca83dc0911c898fbe4bba464fa450a98c4e7a0
ARG DIACRITIZER_REVISION=b806189fe1fc0085b1012b7560ffb5e8ecfd72a2

ADD --checksum=sha256:c3fddc4f139a045b0c4902c57410f0694f1c2fdf9b6939fbe38b1aeae7cd14ba \
     "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-x64-${ORT_VERSION}.tgz" \
     /tmp/ort.tgz

# The glob keeps the whole libonnxruntime.so -> .so.1 -> .so.<version> chain: the
# SONAME is the middle link, so dropping it leaves a library the loader cannot
# resolve. It does not match libonnxruntime_providers_*, which is unused here.
RUN mkdir -p /out/lib \
     && tar -xzf /tmp/ort.tgz -C /out/lib --strip-components=2 --wildcards '*/lib/libonnxruntime.so*'

ADD --chmod=644 --checksum=sha256:2489ccaf7a2a8cba57011b56f7479a407ad6c21e7a93eddcf62e4788f5eeae4b \
     "https://huggingface.co/thewh1teagle/matcha-tts/resolve/${VOICE_REVISION}/matcha-he-en.onnx" \
     /out/models/
ADD --chmod=644 --checksum=sha256:c1fa2624b1e8202a0c0a23259b560b0c41ad92a3a6750bd0e322ce5a2b1acdb6 \
     "https://huggingface.co/Phonikud/phonikud-onnx/resolve/${DIACRITIZER_REVISION}/phonikud-1.0.onnx" \
     /out/models/

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGo is required, unlike most Go services: onnxruntime_go binds the ONNX Runtime
# C API, so there is no static, libc-free build to be had.
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/rg-language .

# Not scratch, and not distroless/cc-debian12 either: ONNX Runtime is a glibc C++
# library, and distroless/cc ships every library the .so lists as NEEDED yet still
# segfaults inside CreateEnv.
FROM debian:bookworm-slim

COPY --from=builder /out/lib/ /usr/local/lib/
COPY --from=builder /out/models/ /models/
COPY --from=builder /out/rg-language /rg-language

EXPOSE 25256
ENTRYPOINT ["/rg-language", "web"]
