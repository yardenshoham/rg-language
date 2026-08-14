// Package diacritizer adds niqqud to plain Hebrew with an ONNX BERT model.
//
// This is the only step in the pipeline that is not deterministic rule-following,
// which is deliberate: all the ambiguity in the language is quarantined here, and
// everything downstream is testable string-to-string code. Where it guesses
// wrong, the fix is a lexicon entry pinning the word's niqqud, not a code change.
package diacritizer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/text/unicode/norm"

	"github.com/yardenshoham/rg-language/pkg/onnx"
)

//go:embed tokenizer.json
var tokenizerJSON []byte

// nikudClasses is the model's output alphabet. Index 0 is "no mark" and index 1
// says the letter is a mater lectionis — a vowel letter that takes no mark of
// its own.
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

// nikudPattern matches everything the model must not see on its input: it is
// trained to add these marks, so any already there are stripped first.
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

	d := &Diacritizer{vocab: make(map[rune]int64, len(tokenizer.Model.Vocab))}
	for token, id := range tokenizer.Model.Vocab {
		switch token {
		case "[UNK]":
			d.unk = id
		case "[CLS]":
			d.cls = id
		case "[SEP]":
			d.sep = id
		}
		if runes := []rune(token); len(runes) == 1 {
			d.vocab[runes[0]] = id
		}
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"nikud_logits", "shin_logits", "additional_logits"}, nil)
	if err != nil {
		return nil, fmt.Errorf("loading diacritizer %s: %w", modelPath, err)
	}
	d.session = session
	return d, nil
}

// Close releases the model.
func (d *Diacritizer) Close() error { return d.session.Destroy() }

// AddDiacritics returns the text with niqqud, plus the stress and vocal-shva
// marks phonikud reads.
func (d *Diacritizer) AddDiacritics(text string) (string, error) {
	var b strings.Builder
	for _, chunk := range chunks(text) {
		out, err := d.predict(chunk)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String(), nil
}

// chunks splits text so no piece exceeds the model's context window, preferring
// to break after a full stop or a newline.
func chunks(text string) []string {
	if len([]rune(text)) <= maxChunkRunes {
		return []string{text}
	}

	var parts []string
	start := 0
	for i, r := range text {
		if r == '.' || r == '\n' {
			parts = append(parts, text[start:i], string(r))
			start = i + len(string(r))
		}
	}
	parts = append(parts, text[start:])

	var out []string
	buf := ""
	for _, part := range parts {
		if len([]rune(buf+part)) <= maxChunkRunes {
			buf += part
			continue
		}
		if buf != "" {
			out = append(out, buf)
		}
		buf = part
	}
	if buf != "" {
		out = append(out, buf)
	}

	// A single sentence can still be too long; cut it to size.
	var sized []string
	for _, chunk := range out {
		runes := []rune(chunk)
		for len(runes) > maxChunkRunes {
			sized = append(sized, string(runes[:maxChunkRunes]))
			runes = runes[maxChunkRunes:]
		}
		sized = append(sized, string(runes))
	}
	return sized
}

// token is one model input position and the span of the sentence it came from,
// in runes.
type token struct {
	id         int64
	start, end int
}

// isAllowed reports whether the model's normalizer keeps a character. Anything
// else is folded into a single unknown token, so that positions stay aligned
// with what the model was trained on.
func isAllowed(r rune) bool {
	switch {
	case r <= 0x007f, r >= 0x0590 && r <= 0x05ff,
		r >= 0x200c && r <= 0x203f, r >= 0x20a0 && r <= 0x20bf,
		r >= 0x2150 && r <= 0x218b, r >= 0x2200 && r <= 0x22ff,
		r >= 0xfb00 && r <= 0xfb4f:
		return true
	}
	return false
}

// fold applies the model's normalizer to a single character: compatibility
// composition, lowercase, then drop combining marks. It reports whether the
// result is a single character the model has an alphabet entry for.
//
// The marks are dropped without decomposing first, which is the detail that
// matters: NFKC composes, so a precomposed ŭ keeps its breve rather than being
// reduced to u, and then falls outside the model's alphabet. Getting this
// backwards changes the token ids and therefore the model's predictions.
//
// On Hebrew that has already had its niqqud removed, all of this is the identity.
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

// tokenize turns the sentence into one token per character.
//
// Two cases are not one-to-one. A character the normalizer deletes outright
// contributes no token at all, and a run of characters outside the model's
// alphabet becomes a single unknown token — both of which keep the sequence
// positions lined up with what the model was trained on.
//
// Because fold works a character at a time rather than over the whole string,
// three exotic inputs give the model a slightly different token sequence than
// upstream would: a compatibility expansion that turns one character into
// several (the ﬁ ligature), a base letter followed by a separate combining
// accent, which upstream would compose first, and an unrepresentable run with a
// combining mark inside it, which splits into two unknown tokens here. None of
// them can lose or duplicate text — the round-trip test covers that — so the
// only effect is on the niqqud predicted around characters that are already
// outside anything this site is for. Closing the gap means tracking an
// alignment from normalized positions back to the original, which is a lot of
// machinery for input no Hebrew sentence contains.
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

func (d *Diacritizer) predict(sentence string) (string, error) {
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
			return "", fmt.Errorf("building diacritizer input: %w", err)
		}
		defer onnx.Destroy(tensor)
		inputs = append(inputs, tensor)
	}

	outputs := []ort.Value{nil, nil, nil}
	if err := d.session.Run(inputs, outputs); err != nil {
		return "", fmt.Errorf("running diacritizer: %w", err)
	}
	for _, output := range outputs {
		defer onnx.Destroy(output)
	}

	nikudLogits, err := logits(outputs[0])
	if err != nil {
		return "", err
	}
	shinLogits, err := logits(outputs[1])
	if err != nil {
		return "", err
	}
	additionalLogits, err := logits(outputs[2])
	if err != nil {
		return "", err
	}

	var b strings.Builder
	prev := 0
	for i, t := range tokens {
		if t.start > prev {
			// Anything between the last token and this one had no token of its
			// own — the normalizer deleted it — so it passes through untouched.
			b.WriteString(string(runes[prev:t.start]))
			prev = t.start
		}
		if t.end-t.start != 1 {
			// A token spanning a whole run of unrepresentable characters. Leave
			// prev where it is so the run is emitted once, by the next gap or by
			// the tail below.
			continue
		}
		char := runes[t.start]
		prev = t.end
		if char < alef || char > tav {
			b.WriteRune(char)
			continue
		}

		mark := nikudClasses[argmax(nikudLogits.at(i))]
		if mark == matresLectionis {
			// A mater lectionis takes no mark, and the model must not put one on
			// a letter that cannot be one.
			if char != 'א' && char != 'ו' && char != 'י' {
				mark = ""
			} else {
				b.WriteRune(char)
				continue
			}
		}

		b.WriteRune(char)
		if char == 'ש' {
			b.WriteString(shinClasses[argmax(shinLogits.at(i))])
		}
		b.WriteString(mark)

		// Each slot of additional_logits is its own binary classifier.
		extra := additionalLogits.at(i)
		for slot, added := range []string{stressChar, vocalShvaChar, prefixChar} {
			if extra[slot] > 0 {
				b.WriteString(added)
			}
		}
	}
	b.WriteString(string(runes[prev:]))
	return b.String(), nil
}

// logitRows is a [1, tokens, classes] output tensor, addressed by token.
type logitRows struct {
	data    []float32
	classes int
}

func (l logitRows) at(token int) []float32 {
	return l.data[token*l.classes : (token+1)*l.classes]
}

func logits(value ort.Value) (logitRows, error) {
	tensor, ok := value.(*ort.Tensor[float32])
	if !ok {
		return logitRows{}, fmt.Errorf("diacritizer returned %T, want a float32 tensor", value)
	}
	shape := tensor.GetShape()
	if len(shape) != 3 {
		return logitRows{}, fmt.Errorf("diacritizer output has shape %v, want three dimensions", shape)
	}
	return logitRows{data: tensor.GetData(), classes: int(shape[2])}, nil
}

func argmax(row []float32) int {
	best := 0
	for i, v := range row {
		if v > row[best] {
			best = i
		}
	}
	return best
}
