package pipeline_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yardenshoham/rg-language/pkg/diacritizer"
	"github.com/yardenshoham/rg-language/pkg/pipeline"
)

// The corpus lives with the transducer it mostly exists to pin. This test needs
// it too, for the one stage the transducer cannot check: the model.
const corpusPath = "../phonikud/testdata/corpus.jsonl"

// The diacritizer is a 300 MB model and 5,012 items take minutes, so by default
// this walks a deterministic sample. Set RG_FULL_CORPUS=1 to check all of them.
const sampleSize = 200

type item struct {
	Text      string `json:"text"`
	Raw       string `json:"raw"`
	Vocalized string `json:"vocalized"`
}

func loadCorpus(t *testing.T) []item {
	t.Helper()
	f, err := os.Open(corpusPath)
	if err != nil {
		t.Fatalf("opening corpus: %v", err)
	}
	defer f.Close()

	var items []item
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var i item
		if err := json.Unmarshal(scanner.Bytes(), &i); err != nil {
			t.Fatalf("decoding corpus: %v", err)
		}
		items = append(items, i)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading corpus: %v", err)
	}
	return items
}

// modelsDir finds the checkpoints, or skips: they are not in the repo.
func modelsDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("RG_MODELS_DIR")
	if dir == "" {
		dir = "/models"
	}
	if _, err := os.Stat(filepath.Join(dir, pipeline.DiacritizerModel)); err != nil {
		t.Skipf("no diacritizer in %s, set RG_MODELS_DIR: %v", dir, err)
	}
	return dir
}

// TestDiacritizerCorpus is the one stage the transducer's own corpus test cannot
// cover, because it runs a model. Given identical token ids the argmax outputs
// are stable, so this is still an exact comparison — a mismatch means the
// tokenizer or the reassembly is wrong, not the weights.
func TestDiacritizerCorpus(t *testing.T) {
	t.Parallel()
	dir := modelsDir(t)

	d, err := diacritizer.New(t.Context(), filepath.Join(dir, pipeline.DiacritizerModel))
	if err != nil {
		t.Fatalf("loading diacritizer: %v", err)
	}
	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Errorf("closing diacritizer: %v", err)
		}
	})

	items := loadCorpus(t)
	step := 1
	if os.Getenv("RG_FULL_CORPUS") == "" {
		step = max(1, len(items)/sampleSize)
	}

	checked, mismatches := 0, 0
	for i := 0; i < len(items); i += step {
		it := items[i]
		checked++

		raw, err := d.AddDiacritics(it.Text)
		if err != nil {
			t.Fatalf("%q: %v", it.Text, err)
		}
		if raw != it.Raw {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("AddDiacritics(%q)\n  got  %q\n  want %q", it.Text, raw, it.Raw)
			}
			continue
		}
		if got := pipeline.ApplyLexicon(pipeline.NormalizeNiqqud(raw)); got != it.Vocalized {
			mismatches++
			if mismatches <= 10 {
				t.Errorf("normalizing %q\n  got  %q\n  want %q", it.Text, got, it.Vocalized)
			}
		}
	}
	if mismatches > 0 {
		t.Errorf("%d of %d checked items differ", mismatches, checked)
	}
	t.Logf("checked %d of %d corpus items", checked, len(items))
}

func TestDoubledVowel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ipa  string
		want bool
	}{
		{"ʔaʁuuɡa", true},  // the artifact NormalizeNiqqud repairs
		{"ʔooʁez", true},   // the same, spelled with a holam
		{"ʔaʁuɡˈa", false}, // once repaired
		{"ʃalˈom", false},
		{"", false},
		{"ʃaʁɡalˈoʁɡom", false}, // the transform never doubles a vowel adjacently
	}
	for _, tt := range tests {
		t.Run(tt.ipa, func(t *testing.T) {
			t.Parallel()
			if got := pipeline.DoubledVowel(tt.ipa); got != tt.want {
				t.Errorf("DoubledVowel(%q) = %v, want %v", tt.ipa, got, tt.want)
			}
		})
	}
}

// knownDoubledVowels are the corpus items the detector flags after
// NormalizeNiqqud has already run. All three are diacritizer artifacts of a
// different shape from the vav one — a vocal shva next to a segol, and the
// letters of an embedded "WiFi" falling through — so there is nothing here to
// repair. The test pins the count: a new one means new vocabulary needs looking at.
var knownDoubledVowels = map[string]bool{
	"איפה אני יכול למצוא רשת WiFi פתוחה ?": true,
	"היא חבקה את התינוק לחזה.":             true,
	"לך באיטיות, ואני אדביק אותך.":         true,
}

// TestCorpusDoubledVowels runs the detector over the whole corpus. It needs no
// models: the IPA is recorded.
func TestCorpusDoubledVowels(t *testing.T) {
	t.Parallel()
	f, err := os.Open(corpusPath)
	if err != nil {
		t.Fatalf("opening corpus: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	total, flagged := 0, 0
	for scanner.Scan() {
		var row struct {
			Text string `json:"text"`
			IPA  string `json:"ipa"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("decoding corpus: %v", err)
		}
		total++
		if !pipeline.DoubledVowel(row.IPA) {
			continue
		}
		flagged++
		if !knownDoubledVowels[row.Text] {
			t.Errorf("new doubled vowel in %q: %q", row.Text, row.IPA)
		}
	}
	if flagged != len(knownDoubledVowels) {
		t.Errorf("%d of %d items flagged, want the %d known ones",
			flagged, total, len(knownDoubledVowels))
	}
}
