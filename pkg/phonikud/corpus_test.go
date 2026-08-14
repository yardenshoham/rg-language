package phonikud_test

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"github.com/yardenshoham/rg-language/pkg/heb"
	"github.com/yardenshoham/rg-language/pkg/phonikud"
	"github.com/yardenshoham/rg-language/pkg/rg"
)

// CorpusPath is the differential corpus: 5,012 Hebrew words and sentences run
// through the original Python implementation once, recording what every stage
// produced. Hebrew -> niqqud -> IPA -> RG is deterministic string-to-string, so
// this pins the port byte-for-byte. The fork is frozen, so the corpus stays
// valid: if a rule here ever drifts, this test says so.
const CorpusPath = "testdata/corpus.jsonl"

// Item is one row of the corpus. IPAExpander is only present where phonikud's
// number-and-date expander would have changed the result; this port skips the
// expander, so it is recorded but not asserted on.
type Item struct {
	Text        string `json:"text"`
	Raw         string `json:"raw"`
	Vocalized   string `json:"vocalized"`
	IPA         string `json:"ipa"`
	RG          string `json:"rg"`
	HebRG       string `json:"heb_rg"`
	Latin       string `json:"latin"`
	IPAExpander string `json:"ipa_exp,omitempty"`
}

// LoadCorpus reads the corpus from path, which callers outside this package
// reach as "../phonikud/" + CorpusPath.
func LoadCorpus(tb testing.TB, path string) []Item {
	tb.Helper()
	f, err := os.Open(path)
	if err != nil {
		tb.Fatalf("opening corpus: %v", err)
	}
	defer f.Close()

	var items []Item
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var item Item
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			tb.Fatalf("decoding corpus line %d: %v", len(items)+1, err)
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		tb.Fatalf("reading corpus: %v", err)
	}
	if len(items) == 0 {
		tb.Fatal("corpus is empty")
	}
	return items
}

// TestCorpus checks every deterministic stage of the pipeline at once. It
// reports up to a few failures per stage rather than one, because a rule that
// regresses usually breaks a whole class of words and the pattern is the
// diagnosis.
func TestCorpus(t *testing.T) {
	t.Parallel()
	items := LoadCorpus(t, CorpusPath)

	stages := []struct {
		name string
		want func(Item) string
		got  func(Item) string
	}{
		{"phonemize", func(i Item) string { return i.IPA },
			func(i Item) string { return phonikud.Phonemize(i.Vocalized) }},
		{"transform", func(i Item) string { return i.RG },
			func(i Item) string { return rg.Transform(i.IPA, rg.StressFirst) }},
		{"hebrew", func(i Item) string { return i.HebRG },
			func(i Item) string { return heb.RG(i.Vocalized) }},
		{"latin", func(i Item) string { return i.Latin },
			func(i Item) string { return heb.Latin(i.RG) }},
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
	items := LoadCorpus(b, CorpusPath)
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		phonikud.Phonemize(items[i%len(items)].Vocalized)
	}
}
