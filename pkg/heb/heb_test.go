package heb_test

import (
	"testing"

	"github.com/yardenshoham/rg-language/pkg/heb"
)

func TestRGReference(t *testing.T) {
	t.Parallel()
	tests := []struct {
		vocalized string
		want      string // unvocalized, as in the project's reference set
	}{
		{"הֵיי", "הרגיי"},
		{"שָׁלוֹם", "שרגלורגום"},
		{"מָה נִשְׁמַע", "מרגה נרגשמרגע"},
		{"הַיּוֹם יוֹם שְׁלִישִׁי", "הרגיורגום יורגום שלירגישירגי"},
		{"גַּנָּן", "גרגנרגן"},
		{"נֶחְמָד", "נרגחמרגד"},
		// פירגיצרגה, not פירגצרגה: the copied vowel keeps the mater yod, which
		// makes the i unambiguous.
		{"אֲנִי מַמָּשׁ אוֹהֵב פִּיצָה", "ארגנירגי מרגמרגש אורגוהרגב פירגיצרגה"},
	}
	for _, tt := range tests {
		t.Run(tt.vocalized, func(t *testing.T) {
			t.Parallel()
			if got := heb.StripMarks(heb.RG(tt.vocalized)); got != tt.want {
				t.Errorf("RG(%q) = %q, want %q", tt.vocalized, got, tt.want)
			}
		})
	}
}

func TestRGVocalized(t *testing.T) {
	t.Parallel()
	tests := []struct{ vocalized, want string }{
		{"מָה נִשְׁמַע", "מָרְגָה נִרְגִשְׁמַרְגַע"},
		{"שָׁלוֹם", "שָׁרְגָלוֹרְגוֹם"},
		{"נֶחְמָד", "נֶרְגֶחְמָרְגָד"},
		{"שְׁלִישִׁי", "שְׁלִירְגִישִׁירְגִי"},
		{"הֵיי", "הֵרְגֵיי"},
	}
	for _, tt := range tests {
		t.Run(tt.vocalized, func(t *testing.T) {
			t.Parallel()
			if got := heb.RG(tt.vocalized); got != tt.want {
				t.Errorf("RG(%q) = %q, want %q", tt.vocalized, got, tt.want)
			}
		})
	}
}

// Concatenating the non-inserted runs must give the input back, rune for rune:
// this is the guard against segment splitting corrupting multi-byte text.
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

func TestLatin(t *testing.T) {
	t.Parallel()
	tests := []struct{ ipa, want string }{
		{"ʃaʁɡalˈoʁɡom", "sha-rga-lo-rgom"},
		{"maʁɡa niʁɡiʃmaʁɡa", "ma-rga ni-rgi-shma-rga"},
		{"ʔaʁɡaniʁɡi", "a-rga-ni-rgi"},
		{"neʁɡeχmaʁɡad", "ne-rge-khma-rgad"},
	}
	for _, tt := range tests {
		t.Run(tt.ipa, func(t *testing.T) {
			t.Parallel()
			if got := heb.Latin(tt.ipa); got != tt.want {
				t.Errorf("Latin(%q) = %q, want %q", tt.ipa, got, tt.want)
			}
		})
	}
}

func TestSyllablesMarkInserted(t *testing.T) {
	t.Parallel()
	words := heb.Syllables("ʃaʁɡalˈoʁɡom")
	if len(words) != 1 {
		t.Fatalf("got %d words, want 1", len(words))
	}
	want := []heb.Syllable{
		{Text: "sha", Stressed: false, Inserted: false},
		{Text: "rga", Stressed: false, Inserted: true},
		{Text: "lo", Stressed: true, Inserted: false},
		{Text: "rgom", Stressed: false, Inserted: true},
	}
	for i, w := range want {
		if words[0][i] != w {
			t.Errorf("syllable %d = %+v, want %+v", i, words[0][i], w)
		}
	}
}
