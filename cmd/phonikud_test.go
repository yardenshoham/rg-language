package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// run fails if the command errors or the output is missing any want.
func run(t *testing.T, stdin string, args []string, want ...string) string {
	t.Helper()
	var out bytes.Buffer
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	for _, w := range want {
		if !strings.Contains(out.String(), w) {
			t.Errorf("%v: output is missing %q:\n%s", args, w, out.String())
		}
	}
	return out.String()
}

func TestPhonikudText(t *testing.T) {
	t.Parallel()
	run(t, "", []string{"phonikud", "שָׁלוֹם"},
		"ipa        ʃalˈom",
		"rg ipa     ʃaʁɡalˈoʁɡom",
		"rg         שרגלורגום",
		"rg niqqud  שָׁרְגָלוֹרְגוֹם",
		"rg latin   sha-rga-lo-rgom",
	)
}

// The JSON field names match the differential corpus so the two can be diffed.
func TestPhonikudJSONReadsStdin(t *testing.T) {
	t.Parallel()
	out := run(t, "גַּנָּן\n\nנֶחְמָד\n", []string{"phonikud", "--json"})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (the blank one should be skipped):\n%s", len(lines), out)
	}
	want := []struct{ vocalized, ipa, hebRG string }{
		{"גַּנָּן", "ɡanˈan", "גַּרְגַנָּרְגָן"},
		{"נֶחְמָד", "neχmˈad", "נֶרְגֶחְמָרְגָד"},
	}
	for i, line := range lines {
		var got struct {
			Vocalized string `json:"vocalized"`
			IPA       string `json:"ipa"`
			HebRG     string `json:"heb_rg"`
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d is not JSON: %v", i, err)
		}
		if got.Vocalized != want[i].vocalized || got.IPA != want[i].ipa || got.HebRG != want[i].hebRG {
			t.Errorf("line %d = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestPhonikudStressModes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ mode, want string }{
		{"first", "ʃaʁɡalˈoʁɡom"},
		{"second", "ʃaʁɡaloʁɡˈom"},
		{"both", "ʃaʁɡalˈoʁɡˈom"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			run(t, "", []string{"phonikud", "--stress", tt.mode, "שָׁלוֹם"}, tt.want)
		})
	}
}

func TestPhonikudRejectsUnknownStress(t *testing.T) {
	t.Parallel()
	root := newRootCmd()
	root.SetArgs([]string{"phonikud", "--stress", "sideways", "שָׁלוֹם"})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.ExecuteContext(t.Context()); err == nil {
		t.Error("an unknown stress mode was accepted")
	}
}

// חכמה needs a kamatz katan the diacritizer cannot emit, so the lexicon pins it.
// Unpinned it has no vowels at all, which --raw shows.
func TestPhonikudRawSkipsLexicon(t *testing.T) {
	t.Parallel()
	run(t, "", []string{"phonikud", "חכמה"}, "ipa        χoχmˈa")
	run(t, "", []string{"phonikud", "--raw", "חכמה"}, "ipa        χˈχm")
}
