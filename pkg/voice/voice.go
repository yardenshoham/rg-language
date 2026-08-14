// Package voice turns IPA phonemes into speech with a Piper VITS checkpoint.
//
// It is fed phonemes, never text: RG output is nonsense words, and any engine
// doing its own text-to-phoneme conversion would "correct" them — which is why
// exact syllable reproduction is achievable at all. Any future voice that cannot
// accept IPA directly is disqualified.
package voice

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/yardenshoham/rg-language/pkg/onnx"
)

//go:embed model.config.json
var configJSON []byte

// Synthesis parameters, not the checkpoint's defaults. A blind order-balanced A/B
// against the defaults was a dead tie (4 "no difference", 2 each way); these are
// kept because all 15 passing human verdicts were collected under them. Changing
// them needs a new listening round — there is no automated substitute.
const (
	noiseScale  = 0.640
	lengthScale = 1.20
	noiseW      = 1.0
)

// Fingerprint identifies the settings above, so audio URLs change if they do.
const Fingerprint = "n0.640-l1.20-w1.0-tail120"

// Appended to every phoneme string before synthesis. Measured: residual energy in
// the final 25 ms frame dropped 61% over a 21-word battery — the model stopped
// ending clips mid-sound — and two human verdicts flipped from bad to ok. It does
// not fix unreleased word-final stops; nothing available does. The trailing
// silence guards against players clipping the last samples.
const (
	tailPhonemes  = " ."
	tailSilenceMS = 120
)

// Piper's symbols: begin, end, and the pad inserted between every phoneme.
const (
	bos = "^"
	eos = "$"
	pad = "_"
)

type config struct {
	Audio struct {
		SampleRate int `json:"sample_rate"`
	} `json:"audio"`
	PhonemeIDMap map[string][]int64 `json:"phoneme_id_map"`
}

// Voice is a loaded synthesis model. It is safe for concurrent use.
type Voice struct {
	session *ort.DynamicAdvancedSession
	cfg     config
}

// New loads the voice checkpoint at modelPath.
func New(ctx context.Context, modelPath string) (*Voice, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := onnx.Init(); err != nil {
		return nil, err
	}

	var cfg config
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return nil, fmt.Errorf("parsing voice config: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath,
		[]string{"input", "input_lengths", "scales"}, []string{"output"}, nil)
	if err != nil {
		return nil, fmt.Errorf("loading voice %s: %w", modelPath, err)
	}
	return &Voice{session: session, cfg: cfg}, nil
}

// Close releases the model.
func (v *Voice) Close() error { return v.session.Destroy() }

// phonemeIDs maps IPA to symbol ids: begin, each phoneme then a pad, end. Unknown
// phonemes are dropped. Iterates runes — every IPA symbol here is multi-byte.
func (v *Voice) phonemeIDs(ipa string) []int64 {
	ids := make([]int64, 0, 2*len([]rune(ipa))+2)
	for _, r := range bos + ipa {
		id, ok := v.cfg.PhonemeIDMap[string(r)]
		if !ok {
			continue
		}
		ids = append(ids, id...)
		ids = append(ids, v.cfg.PhonemeIDMap[pad]...)
	}
	return append(ids, v.cfg.PhonemeIDMap[eos]...)
}

// Synth renders IPA phonemes as a mono 16-bit WAV. Piper is stochastic — the graph
// has a RandomNormalLike node — so callers wanting a phrase to sound the same
// twice must cache the result.
func (v *Voice) Synth(ipa string) ([]byte, error) {
	ids := v.phonemeIDs(ipa + tailPhonemes)

	input, err := ort.NewTensor(ort.NewShape(1, int64(len(ids))), ids)
	if err != nil {
		return nil, fmt.Errorf("building input tensor: %w", err)
	}
	defer onnx.Destroy(input)

	lengths, err := ort.NewTensor(ort.NewShape(1), []int64{int64(len(ids))})
	if err != nil {
		return nil, fmt.Errorf("building length tensor: %w", err)
	}
	defer onnx.Destroy(lengths)

	// The model wants noise, length, noise_w — not the usual prose order.
	scales, err := ort.NewTensor(ort.NewShape(3), []float32{noiseScale, lengthScale, noiseW})
	if err != nil {
		return nil, fmt.Errorf("building scales tensor: %w", err)
	}
	defer onnx.Destroy(scales)

	outputs := []ort.Value{nil}
	if err := v.session.Run([]ort.Value{input, lengths, scales}, outputs); err != nil {
		return nil, fmt.Errorf("running voice: %w", err)
	}
	defer onnx.Destroy(outputs[0])

	samples, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("voice returned %T, want a float32 tensor", outputs[0])
	}
	return wav(samples.GetData(), v.cfg.Audio.SampleRate), nil
}

// wav encodes float samples as a mono 16-bit PCM WAV, clipped to [-1, 1].
func wav(samples []float32, sampleRate int) []byte {
	silence := sampleRate * tailSilenceMS / 1000
	pcm := make([]int16, 0, len(samples)+silence)
	for _, s := range samples {
		pcm = append(pcm, int16(min(max(s, -1), 1)*32767))
	}
	pcm = append(pcm, make([]int16, silence)...)

	dataLen := uint32(len(pcm) * 2)
	var b bytes.Buffer
	b.Grow(44 + int(dataLen))
	for _, field := range []any{
		[4]byte{'R', 'I', 'F', 'F'}, 36 + dataLen, [4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '}, uint32(16), uint16(1), uint16(1),
		uint32(sampleRate), uint32(sampleRate * 2), uint16(2), uint16(16),
		[4]byte{'d', 'a', 't', 'a'}, dataLen, pcm,
	} {
		// A bytes.Buffer never fails to write.
		_ = binary.Write(&b, binary.LittleEndian, field)
	}
	return b.Bytes()
}
