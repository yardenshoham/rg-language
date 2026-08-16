// Package diacritizer adds niqqud to plain Hebrew with an ONNX BERT model. It is the
// pipeline's only non-deterministic step, by design: a wrong guess is fixed in the lexicon.
package diacritizer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/text/unicode/norm"

	"github.com/yardenshoham/rg-language/pkg/onnx"
)

//go:embed tokenizer.json
var tokenizerJSON []byte

// nikudClasses is the model's output alphabet. Index 0 is "no mark"; index 1 says
// the letter is a mater lectionis, which takes no mark of its own.
var nikudClasses = []string{
	"", matresLectionis,
	"\u05bc",
	"\u05b0", "\u05b1", "\u05b2", "\u05b3", "\u05b4", "\u05b5",
	"\u05b6", "\u05b7", "\u05b8", "\u05b9", "\u05ba", "\u05bb",
	"\u05bc\u05b0", "\u05bc\u05b1", "\u05bc\u05b2", "\u05bc\u05b3", "\u05bc\u05b4", "\u05bc\u05b5",
	"\u05bc\u05b6", "\u05bc\u05b7", "\u05bc\u05b8", "\u05bc\u05b9", "\u05bc\u05ba", "\u05bc\u05bb",
	"\u05c7", "\u05bc\u05c7",
}

var shinClasses = []string{"\u05c1", "\u05c2"}

const (
	matresLectionis = "<MAT_LECT>"

	// The three marks phonikud reads on top of standard niqqud.
	stressChar    = "\u05ab" // ole
	vocalShvaChar = "\u05bd" // meteg
	prefixChar    = "|"

	// maxChunkRunes is the model's context window less the two special tokens.
	maxChunkRunes = 2046
)

// nikudPattern matches what the model must not see: it adds these marks itself.
var nikudPattern = regexp.MustCompile(`[\x{0590}-\x{05c7}|]`)

// Diacritizer is a loaded model. It is safe for concurrent use.
type Diacritizer struct {
	session *ort.DynamicAdvancedSession
	vocab   map[string]int64
	unk     int64
	cls     int64
	sep     int64
}

func New(ctx context.Context, modelPath string) (*Diacritizer, error) {
	var tokenizer struct {
		Model struct{ Vocab map[string]int64 }
	}
	if err := json.Unmarshal(tokenizerJSON, &tokenizer); err != nil {
		return nil, fmt.Errorf("parsing tokenizer: %w", err)
	}

	vocab := tokenizer.Model.Vocab
	session, err := onnx.Session(ctx, modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"nikud_logits", "shin_logits", "additional_logits"})
	if err != nil {
		return nil, fmt.Errorf("loading diacritizer %s: %w", modelPath, err)
	}
	return &Diacritizer{session: session, vocab: vocab,
		unk: vocab["[UNK]"], cls: vocab["[CLS]"], sep: vocab["[SEP]"]}, nil
}

func (d *Diacritizer) Close() error { return d.session.Destroy() }

func (d *Diacritizer) AddDiacritics(text string) (string, error) {
	var b strings.Builder
	for _, chunk := range chunks(text) {
		if err := d.predict(&b, chunk); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

// chunks stays under the context window, breaking after a full stop or a newline.
func chunks(text string) []string {
	runes := []rune(text)
	if len(runes) <= maxChunkRunes {
		return []string{text}
	}

	var out []string
	for start := 0; start < len(runes); {
		end := min(start+maxChunkRunes, len(runes)) // a sentence past the window is cut at it
		// Back up to the end of the text, or to either side of a full stop or newline.
		for cut := end; cut > start; cut-- {
			if cut == len(runes) || strings.ContainsRune(".\n", runes[cut]) || strings.ContainsRune(".\n", runes[cut-1]) {
				end = cut
				break
			}
		}
		out = append(out, string(runes[start:end]))
		start = end
	}
	return out
}

// token is one input position and the rune span of the sentence it came from.
type token struct {
	id         int64
	start, end int
}

// allowed is what the model's normalizer keeps. Anything else folds into one unknown
// token, keeping positions aligned with training.
var allowed = &unicode.RangeTable{R16: []unicode.Range16{
	{0x0000, 0x007f, 1}, {0x0590, 0x05ff, 1}, {0x200c, 0x203f, 1}, {0x20a0, 0x20bf, 1},
	{0x2150, 0x218b, 1}, {0x2200, 0x22ff, 1}, {0xfb00, 0xfb4f, 1},
}}

// fold applies the model's normalizer to one character. The order matters: marks are
// dropped WITHOUT decomposing first, so a precomposed ŭ keeps its breve and falls
// outside the alphabet. Reversing that changes the token ids, and so the predictions.
func fold(r rune) (string, bool) {
	folded := strings.Map(func(c rune) rune {
		if unicode.Is(unicode.Mn, c) {
			return -1
		}
		return c
	}, strings.ToLower(norm.NFKC.String(string(r))))
	runes := []rune(folded)
	return folded, len(runes) == 1 && unicode.Is(allowed, runes[0])
}

// tokenize gives each character one token, except that a deleted character gets none and
// an unrepresentable run becomes one unknown token. Accepted divergence from upstream:
// fold works per character, so ligatures split and a run with a mark becomes two unknown
// tokens; the round-trip test proves no text is lost, only niqqud around non-Hebrew text.
func (d *Diacritizer) tokenize(runes []rune) []token {
	tokens := make([]token, 0, len(runes)+2)
	tokens = append(tokens, token{id: d.cls})
	for i := 0; i < len(runes); {
		folded, known := fold(runes[i])
		switch {
		case folded == "":
			i++ // deleted by the normalizer; the text still passes through
		case known:
			id, ok := d.vocab[folded] // known means one rune, so it can only hit a character token
			if !ok {
				id = d.unk
			}
			tokens = append(tokens, token{id: id, start: i, end: i + 1})
			i++
		default:
			run := i + 1
			for ; run < len(runes); run++ {
				if f, ok := fold(runes[run]); f == "" || ok {
					break
				}
			}
			tokens = append(tokens, token{id: d.unk, start: i, end: run})
			i = run
		}
	}
	return append(tokens, token{id: d.sep})
}

func (d *Diacritizer) predict(b *strings.Builder, sentence string) error {
	sentence = nikudPattern.ReplaceAllString(sentence, "")
	runes := []rune(sentence)
	tokens := d.tokenize(runes)

	ids := make([]int64, len(tokens))
	for i, t := range tokens {
		ids[i] = t.id
	}

	shape := ort.NewShape(1, int64(len(tokens)))
	mask, types := slices.Repeat([]int64{1}, len(tokens)), make([]int64, len(tokens))
	var inputs []ort.Value
	for _, data := range [][]int64{ids, mask, types} {
		tensor, err := ort.NewTensor(shape, data)
		if err != nil {
			return fmt.Errorf("building diacritizer input: %w", err)
		}
		defer onnx.Destroy(tensor)
		inputs = append(inputs, tensor)
	}

	outputs := []ort.Value{nil, nil, nil}
	if err := d.session.Run(inputs, outputs); err != nil {
		return fmt.Errorf("running diacritizer: %w", err)
	}
	defer onnx.Destroy(outputs...)

	// Unwrap each [1, tokens, classes] output into a per-token row lookup.
	rows := make([]func(token int) []float32, len(outputs))
	for i, value := range outputs {
		tensor, ok := value.(*ort.Tensor[float32])
		if !ok {
			return fmt.Errorf("diacritizer output %d is %T, want a float32 tensor", i, value)
		}
		shape := tensor.GetShape()
		if len(shape) != 3 {
			return fmt.Errorf("diacritizer output %d has shape %v, want three dimensions", i, shape)
		}
		data, classes := tensor.GetData(), int(shape[2])
		rows[i] = func(token int) []float32 { return data[token*classes : (token+1)*classes] }
	}
	nikudLogits, shinLogits, additionalLogits := rows[0], rows[1], rows[2]

	// marks[i] follows runes[i] in the output, and only a single-rune Hebrew token ever
	// gets one — so the emit below writes every rune once and the rest passes through bare.
	marks := make([]string, len(runes))
	for i, t := range tokens {
		if t.end-t.start != 1 || runes[t.start] < 'א' || runes[t.start] > 'ת' {
			continue
		}
		char := runes[t.start]

		mark := nikudClasses[argmax(nikudLogits(i))]
		if mark == matresLectionis {
			// A mater takes no mark, and only א/ו/י can be one.
			if char == 'א' || char == 'ו' || char == 'י' {
				continue
			}
			mark = ""
		}

		if char == 'ש' {
			mark = shinClasses[argmax(shinLogits(i))] + mark
		}

		// Each slot of additional_logits is its own binary classifier.
		extra := additionalLogits(i)
		for slot, added := range []string{stressChar, vocalShvaChar, prefixChar} {
			if extra[slot] > 0 {
				mark += added
			}
		}
		marks[t.start] = mark
	}

	for i, char := range runes {
		b.WriteRune(char)
		b.WriteString(marks[i])
	}
	return nil
}

func argmax(row []float32) int { return slices.Index(row, slices.Max(row)) }
