package pipeline_test

import (
	"os"
	"testing"

	"github.com/yardenshoham/rg-language/internal/corpustest"
	"github.com/yardenshoham/rg-language/pkg/diacritizer"
	"github.com/yardenshoham/rg-language/pkg/pipeline"
)

const corpusPath = "../phonikud/testdata/corpus.jsonl"

// 5,012 items through a 300 MB model take minutes, so this walks a deterministic
// sample by default. RG_FULL_CORPUS=1 checks all of them.
const sampleSize = 200

// TestDiacritizerCorpus covers the one stage the transducer's own test cannot. Argmax
// over identical token ids is stable, so a mismatch means the tokenizer or the
// reassembly is wrong, not the weights.
func TestDiacritizerCorpus(t *testing.T) {
	t.Parallel()
	d := corpustest.Model(t, pipeline.DiacritizerModel, diacritizer.New)

	items := corpustest.Load(t, corpusPath)
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

// knownDoubledVowels are the items still flagged after NormalizeNiqqud — a different
// shape from the vav artifact, so there is nothing to repair. The count is pinned: a
// new one means new vocabulary to look at.
var knownDoubledVowels = map[string]bool{
	"איפה אני יכול למצוא רשת WiFi פתוחה ?": true,
	"היא חבקה את התינוק לחזה.":             true,
	"לך באיטיות, ואני אדביק אותך.":         true,
}

func TestCorpusDoubledVowels(t *testing.T) {
	t.Parallel()
	items := corpustest.Load(t, corpusPath)

	flagged := 0
	for _, item := range items {
		if !pipeline.DoubledVowel(item.IPA) {
			continue
		}
		flagged++
		if !knownDoubledVowels[item.Text] {
			t.Errorf("new doubled vowel in %q: %q", item.Text, item.IPA)
		}
	}
	if flagged != len(knownDoubledVowels) {
		t.Errorf("%d of %d items flagged, want the %d known ones",
			flagged, len(items), len(knownDoubledVowels))
	}
}
