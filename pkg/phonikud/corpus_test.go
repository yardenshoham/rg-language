package phonikud_test

import (
	"testing"

	"github.com/yardenshoham/rg-language/internal/corpustest"
	"github.com/yardenshoham/rg-language/pkg/heb"
	"github.com/yardenshoham/rg-language/pkg/phonikud"
	"github.com/yardenshoham/rg-language/pkg/rg"
)

const corpusPath = "testdata/corpus.jsonl"

type item = corpustest.Item

// TestCorpus checks every deterministic stage at once, reporting several failures
// per stage: the pattern of a regression is the diagnosis.
func TestCorpus(t *testing.T) {
	t.Parallel()
	items := corpustest.Load(t, corpusPath)

	stages := map[string]func(item) (got, want string){
		"phonemize": func(i item) (string, string) { return phonikud.Phonemize(i.Vocalized), i.IPA },
		"transform": func(i item) (string, string) { return rg.Transform(i.IPA, rg.StressFirst), i.RG },
		"hebrew":    func(i item) (string, string) { return heb.RG(i.Vocalized), i.HebRG },
		"latin":     func(i item) (string, string) { return heb.Latin(i.RG), i.Latin },
	}

	for name, check := range stages {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mismatches := 0
			for _, item := range items {
				got, want := check(item)
				if got == want {
					continue
				}
				mismatches++
				if mismatches <= 10 {
					t.Errorf("%q\n  vocalized %q\n  got       %q\n  want      %q",
						item.Text, item.Vocalized, got, want)
				}
			}
			if mismatches > 0 {
				t.Errorf("%d of %d corpus items differ", mismatches, len(items))
			}
		})
	}
}

// The end-of-word trims, the part the corpus cannot reach: it is diacritizer
// output, and these fire on words the diacritizer left bare.
func TestPhonemizeTrimsWordEndings(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, in, want string }{
		// Two h's: the first trim leaves "ˈh" and the second takes the stray stress with it.
		{"double he", "זהה", "z"},
		{"double he, taf", "תהה", "t"},
		{"double he with a yod", "תהיה", "t"},
		// One h trims alone and leaves the stress — the state above cleans up.
		{"single he", "גה", "ɡˈ"},
		{"two he", "הה", "hˈ"},
		{"three he", "ההה", "h"},
		// A word that ends in a real vowel keeps everything.
		{"vocalized", "שָׁלוֹם", "ʃalˈom"},
		{"vocalized he", "מָה", "mˈa"},
		// The glottal stop goes too.
		{"final alef", "מָא", "mˈa"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := phonikud.Phonemize(tt.in); got != tt.want {
				t.Errorf("Phonemize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
