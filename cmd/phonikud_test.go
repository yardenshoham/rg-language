package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func run(t *testing.T, stdin string, args ...string) string {
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
	return out.String()
}

func TestPhonikudText(t *testing.T) {
	t.Parallel()
	got := run(t, "", "phonikud", "שָׁלוֹם")
	for _, want := range []string{
		"ipa        ʃalˈom",
		"rg ipa     ʃaʁɡalˈoʁɡom",
		"rg         שרגלורגום",
		"rg niqqud  שָׁרְגָלוֹרְגוֹם",
		"rg latin   sha-rga-lo-rgom",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
}

// The JSON field names match the differential corpus so the two can be diffed.
func TestPhonikudJSONReadsStdin(t *testing.T) {
	t.Parallel()
	out := run(t, "גַּנָּן\n\nנֶחְמָד\n", "phonikud", "--json")

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
	tests := []struct{ mode, want string }{
		{"first", "ʃaʁɡalˈoʁɡom"},
		{"second", "ʃaʁɡaloʁɡˈom"},
		{"both", "ʃaʁɡalˈoʁɡˈom"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			got := run(t, "", "phonikud", "--stress", tt.mode, "שָׁלוֹם")
			if !strings.Contains(got, tt.want) {
				t.Errorf("--stress %s did not produce %q:\n%s", tt.mode, tt.want, got)
			}
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
	if got := run(t, "", "phonikud", "חכמה"); !strings.Contains(got, "ipa        χoχmˈa") {
		t.Errorf("the lexicon did not pin חכמה:\n%s", got)
	}
	if got := run(t, "", "phonikud", "--raw", "חכמה"); !strings.Contains(got, "ipa        χˈχm") {
		t.Errorf("--raw still applied the lexicon:\n%s", got)
	}
}
