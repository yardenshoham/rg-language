package phonikud_test

import (
	"testing"

	"github.com/yardenshoham/rg-language/internal/corpustest"
	"github.com/yardenshoham/rg-language/pkg/heb"
	"github.com/yardenshoham/rg-language/pkg/phonikud"
	"github.com/yardenshoham/rg-language/pkg/rg"
)

// The corpus lives with the transducer it mostly exists to pin.
const corpusPath = "testdata/corpus.jsonl"

// item keeps the stages table below readable.
type item = corpustest.Item

// TestCorpus checks every deterministic stage at once, reporting several failures
// per stage: a regressed rule breaks a class of words, and the pattern is the
// diagnosis.
func TestCorpus(t *testing.T) {
	t.Parallel()
	items := corpustest.Load(t, corpusPath)

	stages := []struct {
		name string
		want func(item) string
		got  func(item) string
	}{
		{"phonemize", func(i item) string { return i.IPA },
			func(i item) string { return phonikud.Phonemize(i.Vocalized) }},
		{"transform", func(i item) string { return i.RG },
			func(i item) string { return rg.Transform(i.IPA, rg.StressFirst) }},
		{"hebrew", func(i item) string { return i.HebRG },
			func(i item) string { return heb.RG(i.Vocalized) }},
		{"latin", func(i item) string { return i.Latin },
			func(i item) string { return heb.Latin(i.RG) }},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			t.Parallel()
			mismatches := 0
			for _, item := range items {
				got, want := stage.got(item), stage.want(item)
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

func BenchmarkPhonemize(b *testing.B) {
	items := corpustest.Load(b, corpusPath)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		phonikud.Phonemize(items[i%len(items)].Vocalized)
	}
}
