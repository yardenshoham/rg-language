package rg_test

import (
	"strings"
	"testing"

	"github.com/yardenshoham/rg-language/pkg/rg"
)

// The reference set the project is defined by. Stress is ignored when matching,
// exactly as the original self-test does.
var examples = []struct {
	hebrew string
	ipa    string
	want   string
}{
	{"היי", "hej", "heʁɡej"},
	{"שלום", "ʃalom", "ʃaʁɡaloʁɡom"},
	{"מה נשמע", "ma niʃma", "maʁɡa niʁɡiʃmaʁɡa"},
	{"היום יום שלישי", "hajom jom ʃliʃi", "haʁɡajoʁɡom joʁɡom ʃliʁɡiʃiʁɡi"},
	{"גנן", "ɡanan", "ɡaʁɡanaʁɡan"},
	{"ערוגה", "ʔaʁuɡa", "ʔaʁɡaʁuʁɡuɡaʁɡa"},
	{"נחמד", "neχmad", "neʁɡeχmaʁɡad"},
	{"אני ממש אוהב פיצה", "ʔani mamaʃ ʔohev pitsa", "ʔaʁɡaniʁɡi maʁɡamaʁɡaʃ ʔoʁɡoheʁɡev piʁɡitsaʁɡa"},
}

func TestTransformReference(t *testing.T) {
	t.Parallel()
	for _, e := range examples {
		t.Run(e.hebrew, func(t *testing.T) {
			t.Parallel()
			got := strings.ReplaceAll(rg.Transform(e.ipa, rg.StressFirst), rg.Stress, "")
			if got != e.want {
				t.Errorf("Transform(%q) = %q, want %q", e.ipa, got, e.want)
			}
		})
	}
}

func TestStressModes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode rg.StressMode
		want string
	}{
		{rg.StressFirst, "ʃaʁɡalˈoʁɡom"},
		{rg.StressSecond, "ʃaʁɡaloʁɡˈom"},
		{rg.StressBoth, "ʃaʁɡalˈoʁɡˈom"},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()
			if got := rg.Transform("ʃalˈom", tt.mode); got != tt.want {
				t.Errorf("Transform(mode=%s) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestTransformSegments(t *testing.T) {
	t.Parallel()
	segs := rg.TransformSegments("ʃalˈom", rg.StressFirst)
	want := []rg.Segment{
		{Text: "ʃa", Inserted: false},
		{Text: "ʁɡa", Inserted: true},
		{Text: "lˈo", Inserted: false},
		{Text: "ʁɡo", Inserted: true},
		{Text: "m", Inserted: false},
	}
	if len(segs) != len(want) {
		t.Fatalf("got %d segments %v, want %d", len(segs), segs, len(want))
	}
	for i := range want {
		if segs[i] != want[i] {
			t.Errorf("segment %d = %+v, want %+v", i, segs[i], want[i])
		}
	}
}

// Every rune of the input must survive, in order, once the inserted runs are
// dropped. Nothing here may corrupt multi-byte IPA.
func TestSegmentsPreserveInput(t *testing.T) {
	t.Parallel()
	for _, e := range examples {
		t.Run(e.hebrew, func(t *testing.T) {
			t.Parallel()
			var original strings.Builder
			for _, s := range rg.TransformSegments(e.ipa, rg.StressFirst) {
				if !s.Inserted {
					original.WriteString(s.Text)
				}
			}
			if got := original.String(); got != e.ipa {
				t.Errorf("uninserted text = %q, want %q", got, e.ipa)
			}
		})
	}
}
