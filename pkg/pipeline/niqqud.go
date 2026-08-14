package pipeline

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

//go:embed lexicon.json
var lexiconJSON []byte

// Marks phonikud recognises: standard niqqud, ole, masora, the prefix bar and
// the geresh.
const markClass = `\x{05ab}\x{05af}\x{05b0}-\x{05c7}|'`

var (
	// clusterPattern walks the text one character at a time, gathering the marks
	// stacked on each.
	clusterPattern = regexp.MustCompile(`(?s)(.)([` + markClass + `]*)`)
	markPattern    = regexp.MustCompile(`[` + markClass + `]`)

	// lexicon pins the niqqud of words the diacritizer gets wrong. Overriding at
	// the niqqud level rather than at the IPA level means the phonemes, the RG
	// transform and the Hebrew rendering all stay on the normal path. It is baked
	// into the binary and changed by redeploying — this is the intended fix for
	// every future diacritizer error.
	lexicon = func() map[string]string {
		var entries map[string]string
		if err := json.Unmarshal(lexiconJSON, &entries); err != nil {
			panic("lexicon.json is not valid JSON: " + err.Error())
		}
		return entries
	}()
)

const (
	qubuts   = '\u05bb'
	holam    = '\u05b9'
	vavHolam = '\u05ba'
	dagesh   = '\u05bc'
)

func isHebrewLetter(r rune) bool { return r >= 'א' && r <= 'ת' }

// NormalizeNiqqud repairs a diacritizer artifact that makes a single vowel count
// twice.
//
// phonikud reads ו+dagesh as shuruk /u/ and ו+holam as holam male /o/ — the vav
// itself carries the vowel. When the diacritizer also puts a qubuts or holam on
// the consonant before it, that vowel is emitted twice:
//
//	ערוגה -> ʔaʁuuɡa   (a qubuts on the resh, then a vav with a dagesh)
//	אורז  -> ʔooʁez    (a holam on the alef, then a vav with a holam)
//
// The vav's own mark is the correct one, so the redundant mark is dropped from
// the preceding consonant. That also restores the standard spelling.
func NormalizeNiqqud(vocalized string) string {
	type unit struct{ char, marks string }
	matches := clusterPattern.FindAllStringSubmatch(vocalized, -1)
	units := make([]unit, 0, len(matches))
	for _, m := range matches {
		units = append(units, unit{char: m[1], marks: m[2]})
	}

	for i := range len(units) - 1 {
		cur, next := units[i], units[i+1]
		if next.char != "ו" || cur.char == "ו" {
			continue
		}
		if runes := []rune(cur.char); len(runes) != 1 || !isHebrewLetter(runes[0]) {
			continue
		}
		// Does the vav carry its own vowel — shuruk or holam male?
		if !strings.ContainsAny(next.marks, string([]rune{dagesh, holam, vavHolam})) {
			continue
		}
		for _, duplicate := range []rune{qubuts, holam} {
			if at := strings.IndexRune(cur.marks, duplicate); at >= 0 {
				units[i].marks = cur.marks[:at] + cur.marks[at+len(string(duplicate)):]
				break
			}
		}
	}

	var b strings.Builder
	for _, u := range units {
		b.WriteString(u.char)
		b.WriteString(u.marks)
	}
	return b.String()
}

// ApplyLexicon replaces each word whose bare spelling is pinned in the lexicon.
func ApplyLexicon(vocalized string) string {
	words := strings.Split(vocalized, " ")
	for i, word := range words {
		if pinned, ok := lexicon[markPattern.ReplaceAllString(word, "")]; ok {
			words[i] = pinned
		}
	}
	return strings.Join(words, " ")
}

// DoubledVowel reports whether the IPA contains two identical adjacent vowels,
// which real Hebrew essentially never produces. It is the detector for the bug
// NormalizeNiqqud repairs: run it over any new vocabulary.
func DoubledVowel(ipa string) bool {
	var prev rune
	for _, r := range ipa {
		if r == prev && strings.ContainsRune("aeiou", r) {
			return true
		}
		prev = r
	}
	return false
}
