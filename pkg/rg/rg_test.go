package rg_test

import (
	"strings"
	"testing"

	"github.com/yardenshoham/rg-language/pkg/rg"
)

// The reference set the project is defined by. Stress is ignored when matching,
// as the original self-test does.
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

func TestIsVowel(t *testing.T) {
	t.Parallel()
	for _, r := range "aeiou" {
		if !rg.IsVowel(r) {
			t.Errorf("IsVowel(%q) = false", r)
		}
	}
	// Consonants the rule must not fire on, including the inserted cluster's own.
	for _, r := range "ʃʁɡχʔʒjlmnstvzbdfhkp" {
		if rg.IsVowel(r) {
			t.Errorf("IsVowel(%q) = true", r)
		}
	}
}
