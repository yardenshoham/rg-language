// Package diacritizer adds niqqud to plain Hebrew with an ONNX BERT model.
//
// It is the pipeline's only non-deterministic step, deliberately: everything
// downstream is string-to-string, and a wrong guess is fixed by a lexicon entry.
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

// shinClasses picks which of the two dots a shin gets.
var shinClasses = []string{"\u05c1", "\u05c2"}

const (
	matresLectionis = "<MAT_LECT>"

	// The three marks phonikud reads on top of standard niqqud.
	stressChar    = "\u05ab" // ole
	vocalShvaChar = "\u05bd" // meteg
	prefixChar    = "|"

	alef = 'א'
	tav  = 'ת'

	// maxChunkRunes is the model's context window less the two special tokens.
	maxChunkRunes = 2046
)

// nikudPattern matches what the model must not see: it adds these marks, so any
// already present are stripped first.
var nikudPattern = regexp.MustCompile(`[\x{0590}-\x{05c7}|]`)

// Diacritizer is a loaded model. It is safe for concurrent use.
type Diacritizer struct {
	session *ort.DynamicAdvancedSession
	vocab   map[rune]int64
	unk     int64
	cls     int64
	sep     int64
}

// New loads the diacritizer at modelPath.
func New(ctx context.Context, modelPath string) (*Diacritizer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := onnx.Init(); err != nil {
		return nil, err
	}

	var tokenizer struct {
		Model struct {
			Vocab map[string]int64 `json:"vocab"`
		} `json:"model"`
	}
	if err := json.Unmarshal(tokenizerJSON, &tokenizer); err != nil {
		return nil, fmt.Errorf("parsing tokenizer: %w", err)
	}

	vocab := tokenizer.Model.Vocab
	chars := make(map[rune]int64, len(vocab))
	for token, id := range vocab {
		if runes := []rune(token); len(runes) == 1 {
			chars[runes[0]] = id
		}
	}

	options, err := onnx.SessionOptions()
	if err != nil {
		return nil, err
	}
	defer func() { _ = options.Destroy() }()

	session, err := ort.NewDynamicAdvancedSession(modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"nikud_logits", "shin_logits", "additional_logits"}, options)
	if err != nil {
		return nil, fmt.Errorf("loading diacritizer %s: %w", modelPath, err)
	}
	return &Diacritizer{session: session, vocab: chars,
		unk: vocab["[UNK]"], cls: vocab["[CLS]"], sep: vocab["[SEP]"]}, nil
}

func (d *Diacritizer) Close() error { return d.session.Destroy() }

// AddDiacritics returns the text with niqqud, stress and vocal-shva marks.
func (d *Diacritizer) AddDiacritics(text string) (string, error) {
	var b strings.Builder
	for _, chunk := range chunks(text) {
		if err := d.predict(&b, chunk); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

// chunks splits text so no piece exceeds the model's context window, preferring
// to break after a full stop or a newline.
func chunks(text string) []string {
	runes := []rune(text)
	if len(runes) <= maxChunkRunes {
		return []string{text}
	}

	// Where a chunk may start, for 1 <= i <= len(runes): the end of the text, or
	// either side of a full stop or newline.
	breakable := func(i int) bool {
		return i == len(runes) || runes[i] == '.' || runes[i] == '\n' ||
			runes[i-1] == '.' || runes[i-1] == '\n'
	}

	var out []string
	for start := 0; start < len(runes); {
		full := min(start+maxChunkRunes, len(runes))
		end := full
		for end > start && !breakable(end) {
			end--
		}
		if end == start {
			end = full // a sentence longer than the window: cut it at the window
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

// isAllowed reports whether the model's normalizer keeps a character. Anything
// else folds into one unknown token, keeping positions aligned with training.
func isAllowed(r rune) bool {
	return r <= 0x007f || (r >= 0x0590 && r <= 0x05ff) ||
		(r >= 0x200c && r <= 0x203f) || (r >= 0x20a0 && r <= 0x20bf) ||
		(r >= 0x2150 && r <= 0x218b) || (r >= 0x2200 && r <= 0x22ff) ||
		(r >= 0xfb00 && r <= 0xfb4f)
}

// fold applies the model's normalizer to one character — NFKC, lowercase, drop
// combining marks — and reports whether the result is a single character in the
// model's alphabet. The order matters: marks are dropped WITHOUT decomposing
// first, so a precomposed ŭ keeps its breve and falls outside the alphabet.
// Reversing that changes the token ids, and so the model's predictions.
func fold(r rune) (string, bool) {
	var b strings.Builder
	for _, c := range strings.ToLower(norm.NFKC.String(string(r))) {
		if !unicode.Is(unicode.Mn, c) {
			b.WriteRune(c)
		}
	}
	folded := b.String()
	runes := []rune(folded)
	return folded, len(runes) == 1 && isAllowed(runes[0])
}

// tokenize turns the sentence into one token per character. Two cases are not
// one-to-one: a character the normalizer deletes gets no token, and a run outside
// the model's alphabet becomes one unknown token.
//
// Known, accepted divergence from upstream: fold works per character, so the ﬁ
// ligature splits, a base letter plus a combining accent is not composed first, and
// an unrepresentable run containing a mark becomes two unknown tokens where upstream
// makes one. None can lose or duplicate text — the round-trip test covers that — so
// the only effect is on niqqud around characters this site is not for.
func (d *Diacritizer) tokenize(runes []rune) []token {
	tokens := make([]token, 0, len(runes)+2)
	tokens = append(tokens, token{id: d.cls})
	for i := 0; i < len(runes); {
		folded, known := fold(runes[i])
		switch {
		case folded == "":
			i++ // deleted by the normalizer; the text still passes through
		case known:
			id, ok := d.vocab[[]rune(folded)[0]]
			if !ok {
				id = d.unk
			}
			tokens = append(tokens, token{id: id, start: i, end: i + 1})
			i++
		default:
			run := i + 1
			for run < len(runes) {
				if f, ok := fold(runes[run]); f == "" || ok {
					break
				}
				run++
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
	mask := make([]int64, len(tokens))
	for i, t := range tokens {
		ids[i], mask[i] = t.id, 1
	}
	types := make([]int64, len(tokens))

	shape := ort.NewShape(1, int64(len(tokens)))
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
	for _, output := range outputs {
		defer onnx.Destroy(output)
	}

	rows, err := logits(outputs)
	if err != nil {
		return err
	}
	nikudLogits, shinLogits, additionalLogits := rows[0], rows[1], rows[2]

	// marks[i] follows runes[i] in the output, and only a single-rune token in the
	// Hebrew alphabet ever gets one — so the emit below writes every rune exactly
	// once and everything else passes through bare.
	marks := make([]string, len(runes))
	for i, t := range tokens {
		if t.end-t.start != 1 {
			continue
		}
		char := runes[t.start]
		if char < alef || char > tav {
			continue
		}

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

// logits unwraps each [1, tokens, classes] tensor into a per-token row lookup.
func logits(values []ort.Value) ([]func(token int) []float32, error) {
	rows := make([]func(token int) []float32, len(values))
	for i, value := range values {
		tensor, ok := value.(*ort.Tensor[float32])
		if !ok {
			return nil, fmt.Errorf("diacritizer returned %T, want a float32 tensor", value)
		}
		shape := tensor.GetShape()
		if len(shape) != 3 {
			return nil, fmt.Errorf("diacritizer output has shape %v, want three dimensions", shape)
		}
		data, classes := tensor.GetData(), int(shape[2])
		rows[i] = func(token int) []float32 { return data[token*classes : (token+1)*classes] }
	}
	return rows, nil
}

func argmax(row []float32) int { return slices.Index(row, slices.Max(row)) }
