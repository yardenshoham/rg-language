package diacritizer

import (
	"strings"
	"testing"

	"github.com/yardenshoham/rg-language/internal/corpustest"
)

// Chunking must never lose or duplicate text, nor exceed the window. These shapes
// are the ones that break naive splitters.
func TestChunks(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, text string }{
		{"short", "שלום עולם"},
		{"empty", ""},
		{"exactly at the limit", strings.Repeat("ג", maxChunkRunes)},
		{"one over the limit", strings.Repeat("ב", maxChunkRunes+1)},
		{"many sentences", strings.Repeat("שלום עולם. ", 400)},
		{"one huge sentence", strings.Repeat("א", 5000)},
		{"newlines only", strings.Repeat("ד\n", 1500)},
		{"trailing separator", strings.Repeat("ה", 3000) + "."},
		{"separators only", strings.Repeat(".", 3000)},
		{"mixed separators", strings.Repeat("ו. \nז", 700)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := chunks(tt.text)
			if joined := strings.Join(got, ""); joined != tt.text {
				t.Errorf("chunks lost or duplicated text: %d runes in, %d out",
					len([]rune(tt.text)), len([]rune(joined)))
			}
			for i, chunk := range got {
				if n := len([]rune(chunk)); n > maxChunkRunes {
					t.Errorf("chunk %d has %d runes, over the %d limit", i, n, maxChunkRunes)
				}
			}
		})
	}
}

// Fold order is load-bearing: decomposing first would silently change predictions.
func TestFold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		in    rune
		out   string
		known bool
	}{
		{"hebrew letter is untouched", 'ש', "ש", true},
		{"final letter is untouched", 'ם', "ם", true},
		{"ascii is untouched", 'a', "a", true},
		{"latin is lowercased", 'A', "a", true},
		{"digit is kept", '7', "7", true},
		{"space is kept", ' ', " ", true},
		{"precomposed u with breve stays out of the alphabet", 'ŭ', "ŭ", false},
		{"precomposed e acute stays out of the alphabet", 'é', "é", false},
		{"emoji is out of the alphabet", '😀', "😀", false},
		{"a lone combining mark is deleted outright", '́', "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out, known := fold(tt.in)
			if out != tt.out || known != tt.known {
				t.Errorf("fold(%q) = (%q, %v), want (%q, %v)", tt.in, out, known, tt.out, tt.known)
			}
		})
	}
}

// On Hebrew the tokens are one per rune, which is what makes the reassembly simple.
// Anything unrepresentable collapses into one token so positions stay put.
func TestTokenize(t *testing.T) {
	t.Parallel()
	d := &Diacritizer{vocab: map[string]int64{"ש": 234, "ל": 221, "ו": 214, "ם": 225},
		unk: 0, cls: 1, sep: 2}

	tokens := d.tokenize([]rune("שלום"))
	if len(tokens) != 6 {
		t.Fatalf("got %d tokens, want 4 letters plus the two special ones: %+v", len(tokens), tokens)
	}
	if tokens[0].id != d.cls || tokens[len(tokens)-1].id != d.sep {
		t.Errorf("the sequence is not wrapped in [CLS] and [SEP]: %+v", tokens)
	}
	for i, want := range []int64{234, 221, 214, 225} {
		if got := tokens[i+1]; got.id != want || got.start != i || got.end != i+1 {
			t.Errorf("token %d = %+v, want id %d spanning rune %d", i+1, got, want, i)
		}
	}

	tokens = d.tokenize([]rune("ש😀😀😀ל"))
	if len(tokens) != 5 {
		t.Fatalf("got %d tokens, want the emoji run collapsed into one: %+v", len(tokens), tokens)
	}
	if run := tokens[2]; run.id != d.unk || run.start != 1 || run.end != 4 {
		t.Errorf("emoji run = %+v, want one unknown token spanning runes 1..4", run)
	}
}

// Whatever the model decides, stripping the niqqud again must return every input character
// exactly once — the guard against reassembly losing text around non-one-per-rune tokens.
func TestReassemblyRoundTrip(t *testing.T) {
	t.Parallel()
	d := corpustest.Model(t, "phonikud-1.0.onnx", New)

	for _, in := range []string{
		"שלום עולם",
		"שלום 😀 עולם",
		"😀😀😀 שלום",                        // a run at the start
		"שלום 😀😀😀",                        // a run at the end, where only the tail write can emit it
		"😀",                               // nothing but a run
		"שלום 🇮🇱 עולם",                    // one flag, several code points
		"anstataŭi במובן של anstataŭigi.", // precomposed letters outside the alphabet
		"שלום é עולם",
		"שלום́עולם",                   // a combining mark the normalizer deletes outright
		"ש\ufe0f\U0001f600\U0001f600", // a deleted mark immediately before a multi-rune unknown run
		"\u0300\U0001f600\U0001f600 שלום",
		`צה"ל`,
		"מה נשמע? 25 שקל!",
		"",
		"   ",
	} {
		out, err := d.AddDiacritics(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got := nikudPattern.ReplaceAllString(out, ""); got != in {
			t.Errorf("round trip changed the text\n  in   %q\n  out  %q\n  bare %q", in, out, got)
		}
	}
}
