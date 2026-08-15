package voice

import (
	"encoding/binary"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/yardenshoham/rg-language/internal/corpustest"
)

// testVoice builds a Voice with a stand-in symbol table and no session, so the
// encoding can be checked without the 131 MB checkpoint.
func testVoice() *Voice {
	return &Voice{
		ids:        map[rune]int64{'ʃ': 96, 'a': 14, 'ʁ': 94, 'ɡ': 66, 'l': 24, 'ˈ': 120, 'o': 27, 'm': 25},
		sampleRate: 22050,
	}
}

// Matcha wants a blank before, between and after every phoneme. This is the whole
// contract between the RG string and the model, and getting it wrong does not fail
// loudly — a checkpoint fed the wrong interleaving produces fluent-sounding nonsense.
func TestPhonemeIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ipa  string
		want []int64
	}{
		{"", []int64{0}},
		{"ʃa", []int64{0, 96, 0, 14, 0}},
		{"ʃaʁɡalˈom", []int64{
			0, 96, 0, 14, 0, 94, 0, 66, 0, 14, 0, 24, 0, 120, 0, 27, 0, 25, 0,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.ipa, func(t *testing.T) {
			t.Parallel()
			if got := testVoice().phonemeIDs(tt.ipa); !slices.Equal(got, tt.want) {
				t.Errorf("phonemeIDs(%q) = %v, want %v", tt.ipa, got, tt.want)
			}
		})
	}
}

// Unknown phonemes must be dropped, not given an arbitrary id.
func TestPhonemeIDsSkipsUnknown(t *testing.T) {
	t.Parallel()
	v := testVoice()
	known := v.phonemeIDs("ʃalom")
	if got := v.phonemeIDs("ʃal☃om"); !slices.Equal(got, known) {
		t.Errorf("an unknown symbol changed the ids: %v", got)
	}
}

// The model pads its output tensor, so the reported length is the real one.
func TestTrim(t *testing.T) {
	t.Parallel()
	samples := []float32{1, 2, 3, 4}
	tests := []struct {
		name    string
		lengths []int64
		want    []float32
	}{
		{"trims to the reported length", []int64{2}, []float32{1, 2}},
		{"a full-length tensor is untouched", []int64{4}, samples},
		{"an over-long length is ignored", []int64{99}, samples},
		{"a negative length is ignored", []int64{-1}, samples},
		{"no length at all is ignored", nil, samples},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := trim(samples, tt.lengths); !slices.Equal(got, tt.want) {
				t.Errorf("trim(%v, %v) = %v, want %v", samples, tt.lengths, got, tt.want)
			}
		})
	}
}

func TestWAVHeader(t *testing.T) {
	t.Parallel()
	const rate = 22050
	// Two samples plus the 120 ms of trailing silence.
	out := wav([]float32{0, 1.5, -1.5}, rate)

	silence := rate * tailSilenceMS / 1000
	wantSamples := 3 + silence
	if got := len(out); got != 44+2*wantSamples {
		t.Fatalf("wav length = %d, want %d", got, 44+2*wantSamples)
	}
	if string(out[:4]) != "RIFF" || string(out[8:12]) != "WAVE" || string(out[36:40]) != "data" {
		t.Fatalf("not a WAV: %q", out[:44])
	}
	if got := binary.LittleEndian.Uint16(out[22:24]); got != 1 {
		t.Errorf("channels = %d, want mono", got)
	}
	if got := binary.LittleEndian.Uint32(out[24:28]); got != rate {
		t.Errorf("sample rate = %d, want %d", got, rate)
	}
	if got := binary.LittleEndian.Uint16(out[34:36]); got != 16 {
		t.Errorf("bit depth = %d, want 16", got)
	}

	// Out-of-range samples clip rather than wrapping around.
	if got := int16(binary.LittleEndian.Uint16(out[46:48])); got != 32767 {
		t.Errorf("1.5 encoded as %d, want it clipped to 32767", got)
	}
	if got := int16(binary.LittleEndian.Uint16(out[48:50])); got != -32767 {
		t.Errorf("-1.5 encoded as %d, want it clipped to -32767", got)
	}
}

// The corpus is the full set of phonemes this pipeline can ever emit, so every one of
// them has to exist in the checkpoint's own table. A missing symbol is dropped in
// silence — the word would simply lose a sound — which is exactly the failure a swap
// to a differently-trained voice would introduce. Needs the model, so it can skip.
func TestCheckpointSaysEverythingWeCanEmit(t *testing.T) {
	t.Parallel()
	dir := corpustest.ModelsDir()
	v, err := New(t.Context(), filepath.Join(dir, "matcha-he-en.onnx"))
	if err != nil {
		t.Skipf("no voice in %s, set RG_MODELS_DIR: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := v.Close(); err != nil {
			t.Errorf("closing voice: %v", err)
		}
	})

	if v.sampleRate != 22050 {
		t.Errorf("sample rate = %d, want 22050", v.sampleRate)
	}

	inventory := map[rune]bool{}
	for _, item := range corpustest.Load(t, "../phonikud/testdata/corpus.jsonl") {
		for _, r := range item.IPA + item.RG {
			inventory[r] = true
		}
	}
	// The tail is synthesized too, so it counts as part of the inventory.
	for _, r := range tailPhonemes {
		inventory[r] = true
	}

	var missing []string
	for r := range inventory {
		if _, ok := v.ids[r]; !ok {
			missing = append(missing, string(r))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("the checkpoint cannot say %d symbol(s) the pipeline emits: %s",
			len(missing), strings.Join(missing, " "))
	}
}
