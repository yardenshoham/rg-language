// Package voice turns IPA phonemes into speech with a Matcha-TTS checkpoint.
//
// It is fed phonemes, never text: RG output is nonsense words, and any engine doing
// its own text-to-phoneme conversion would "correct" them. Any future voice that
// cannot accept IPA directly is disqualified.
package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/yardenshoham/rg-language/pkg/onnx"
)

// Synthesis parameters. A blind eight-voice round — the incumbent Piper checkpoint,
// three other Hebrew Piper voices, Kokoro, Mixer-TTS and Microsoft's he-IL voice, all
// loudness-matched and lettered — was rendered under exactly these, and this voice was
// the only one rated good on every sentence. They are the settings that were judged,
// not the checkpoint's defaults, so changing either needs a new listening round.
const (
	temperature = 0.667
	lengthScale = 0.70
)

// Fingerprint identifies the voice and the settings above, so audio URLs change if any
// of them do.
const Fingerprint = "matcha-t0.667-l0.70-tail120"

// Appended to every phoneme string before synthesis. Measured on the previous voice:
// residual energy in the final 25 ms frame dropped 61% over a 21-word battery — the
// model stopped ending clips mid-sound — and two human verdicts flipped from bad to ok.
// The trailing silence guards against players clipping the last samples.
const (
	tailPhonemes  = " ."
	tailSilenceMS = 120
)

// blank is Matcha's pad symbol, id 0. It goes before, between and after every phoneme:
// the model was trained on that interleaving, and a checkpoint trained without it turns
// into fluent babble at more than twice the right length when given it, so this is not
// a detail that can be carried over between voices by assumption.
const blank = 0

// Voice is a loaded synthesis model. It is safe for concurrent use.
type Voice struct {
	session    *ort.DynamicAdvancedSession
	ids        map[rune]int64
	sampleRate int
}

func New(ctx context.Context, modelPath string) (*Voice, error) {
	session, err := onnx.Session(ctx, modelPath,
		[]string{"x", "x_lengths", "scales"}, []string{"wav", "wav_lengths"})
	if err != nil {
		return nil, fmt.Errorf("loading voice %s: %w", modelPath, err)
	}

	v := &Voice{session: session}
	if err := v.readMetadata(); err != nil {
		_ = session.Destroy()
		return nil, err
	}
	return v, nil
}

// readMetadata takes the symbol table and the sample rate from the checkpoint, which
// is why this package ships no config file: a table carried by the checkpoint it was
// trained with cannot drift apart from it.
func (v *Voice) readMetadata() error {
	meta, err := v.session.GetModelMetadata()
	if err != nil {
		return fmt.Errorf("reading voice metadata: %w", err)
	}
	defer func() { _ = meta.Destroy() }()

	// A key the checkpoint does not carry reads back empty and fails the parse below.
	table, _, _ := meta.LookupCustomMetadataMap("matcha.symbols")

	var symbols []string
	if err := json.Unmarshal([]byte(table), &symbols); err != nil {
		return fmt.Errorf("parsing matcha.symbols, so this is not a Matcha checkpoint: %w", err)
	}
	v.ids = make(map[rune]int64, len(symbols))
	for id, symbol := range symbols {
		// Every symbol in the table is a single rune; anything else could not be
		// looked up by the rune-at-a-time walk in phonemeIDs anyway.
		if runes := []rune(symbol); len(runes) == 1 {
			v.ids[runes[0]] = int64(id)
		}
	}

	rate, _, _ := meta.LookupCustomMetadataMap("matcha.sample_rate")
	if v.sampleRate, err = strconv.Atoi(rate); err != nil {
		return fmt.Errorf("parsing matcha.sample_rate %q: %w", rate, err)
	}
	return nil
}

func (v *Voice) Close() error { return v.session.Destroy() }

// phonemeIDs blank-interleaves the symbol ids, dropping unknowns. Runes, not bytes:
// every IPA symbol here is multi-byte.
func (v *Voice) phonemeIDs(ipa string) []int64 {
	ids := make([]int64, 1, 2*len([]rune(ipa))+1)
	ids[0] = blank
	for _, r := range ipa {
		id, ok := v.ids[r]
		if !ok {
			continue
		}
		ids = append(ids, id, blank)
	}
	return ids
}

// Synth renders IPA phonemes as a mono 16-bit WAV. Matcha samples a noise prior, so
// callers wanting a phrase to sound the same twice must cache the result.
func (v *Voice) Synth(ipa string) ([]byte, error) {
	ids := v.phonemeIDs(ipa + tailPhonemes)

	x, xErr := ort.NewTensor(ort.NewShape(1, int64(len(ids))), ids)
	lengths, lenErr := ort.NewTensor(ort.NewShape(1), []int64{int64(len(ids))})
	scales, scaleErr := ort.NewTensor(ort.NewShape(2), []float32{temperature, lengthScale})
	if err := errors.Join(xErr, lenErr, scaleErr); err != nil {
		return nil, fmt.Errorf("building voice input: %w", err)
	}
	// After the check, never before: Destroy dereferences its receiver.
	inputs := []ort.Value{x, lengths, scales}
	defer onnx.Destroy(inputs...)

	outputs := []ort.Value{nil, nil}
	if err := v.session.Run(inputs, outputs); err != nil {
		return nil, fmt.Errorf("running voice: %w", err)
	}
	defer onnx.Destroy(outputs...)

	samples, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("voice returned %T for wav, want a float32 tensor", outputs[0])
	}
	lens, ok := outputs[1].(*ort.Tensor[int64])
	if !ok {
		return nil, fmt.Errorf("voice returned %T for wav_lengths, want an int64 tensor", outputs[1])
	}
	return wav(trim(samples.GetData(), lens.GetData()), v.sampleRate), nil
}

// trim cuts the padded output tensor down to the samples the model actually produced.
func trim(samples []float32, lengths []int64) []float32 {
	if len(lengths) == 0 || lengths[0] < 0 || int(lengths[0]) >= len(samples) {
		return samples
	}
	return samples[:lengths[0]]
}

// wav encodes float samples as a mono 16-bit PCM WAV, clipped to [-1, 1]. The clip is
// a backstop rather than a working part: the loudest of thirty draws across the
// reference sentences peaked at 0.988.
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
		[]byte("RIFF"), 36 + dataLen, []byte("WAVE"), []byte("fmt "), uint32(16),
		uint16(1), uint16(1), uint32(sampleRate), uint32(sampleRate * 2), uint16(2),
		uint16(16), []byte("data"), dataLen, pcm,
	} {
		// A bytes.Buffer never fails to write.
		_ = binary.Write(&b, binary.LittleEndian, field)
	}
	return b.Bytes()
}
