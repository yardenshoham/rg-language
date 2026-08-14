package phonikud_test

import (
	"testing"

	"github.com/yardenshoham/rg-language/pkg/phonikud"
)

// The end-of-word trims, the part the corpus cannot reach: it is diacritizer
// output, and these fire on words the diacritizer left bare.
func TestPhonemizeTrimsWordEndings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
	}{
		// Two h's: the first trim leaves "ˈh" and the second takes the stray stress
		// with it. Dropping it as redundant leaves a phantom consonant.
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := phonikud.Phonemize(tt.in); got != tt.want {
				t.Errorf("Phonemize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
