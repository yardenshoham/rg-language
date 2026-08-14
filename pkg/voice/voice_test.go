package voice

import (
	"encoding/binary"
	"encoding/json"
	"slices"
	"testing"
)

// testVoice builds a Voice with the embedded config but no session.
func testVoice(t *testing.T) *Voice {
	t.Helper()
	v := &Voice{}
	if err := json.Unmarshal(configJSON, &v.cfg); err != nil {
		t.Fatalf("parsing embedded config: %v", err)
	}
	return v
}

// The id sequences the Python implementation produces for the same phonemes, tail
// included. These proved the port and the original agree at the model's level.
func TestPhonemeIDs(t *testing.T) {
	t.Parallel()
	v := testVoice(t)

	tests := []struct {
		ipa  string
		want []int64
	}{
		{"ʃaʁɡalˈoʁɡom", []int64{
			1, 0, 96, 0, 14, 0, 94, 0, 66, 0, 14, 0, 24, 0, 120, 0, 27, 0,
			94, 0, 66, 0, 27, 0, 25, 0, 3, 0, 10, 0, 2,
		}},
		{"mˈaʁɡa", []int64{1, 0, 25, 0, 120, 0, 14, 0, 94, 0, 66, 0, 14, 0, 3, 0, 10, 0, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.ipa, func(t *testing.T) {
			t.Parallel()
			if got := v.phonemeIDs(tt.ipa + tailPhonemes); !slices.Equal(got, tt.want) {
				t.Errorf("phonemeIDs(%q) = %v, want %v", tt.ipa, got, tt.want)
			}
		})
	}
}

// Unknown phonemes must be dropped, not given an arbitrary id.
func TestPhonemeIDsSkipsUnknown(t *testing.T) {
	t.Parallel()
	v := testVoice(t)
	known := v.phonemeIDs("ʃalom")
	if got := v.phonemeIDs("ʃal☃om"); !slices.Equal(got, known) {
		t.Errorf("an unknown symbol changed the ids: %v", got)
	}
}

func TestSampleRate(t *testing.T) {
	t.Parallel()
	if got := testVoice(t).cfg.Audio.SampleRate; got != 22050 {
		t.Errorf("sample rate = %d, want 22050", got)
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
