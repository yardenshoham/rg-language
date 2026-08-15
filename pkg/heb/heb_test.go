package heb_test

import (
	"testing"

	"github.com/yardenshoham/rg-language/pkg/heb"
)

// Concatenating the non-inserted runs must give the input back, rune for rune:
// the guard against segment splitting corrupting multi-byte text.
func TestRGSegments(t *testing.T) {
	t.Parallel()
	segs := heb.RGSegments("שָׁלוֹם")
	var rebuilt, original string
	for _, s := range segs {
		rebuilt += s.Text
		if !s.Inserted {
			original += s.Text
		}
	}
	if rebuilt != "שָׁרְגָלוֹרְגוֹם" {
		t.Errorf("segments join to %q", rebuilt)
	}
	if original != "שָׁלוֹם" {
		t.Errorf("uninserted text = %q, want the input back", original)
	}
	if len(segs) != 5 {
		t.Errorf("got %d segments, want 5 alternating runs: %+v", len(segs), segs)
	}
}

func TestSyllablesMarkInserted(t *testing.T) {
	t.Parallel()
	words := heb.Syllables("ʃaʁɡalˈoʁɡom")
	if len(words) != 1 {
		t.Fatalf("got %d words, want 1", len(words))
	}
	want := []heb.Syllable{
		{Text: "sha"},
		{Text: "rga", Inserted: true},
		{Text: "lo", Stressed: true},
		{Text: "rgom", Inserted: true},
	}
	for i, w := range want {
		if words[0][i] != w {
			t.Errorf("syllable %d = %+v, want %+v", i, words[0][i], w)
		}
	}
}
