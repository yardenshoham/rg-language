package pipeline

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/yardenshoham/rg-language/pkg/rg"
)

//go:embed lexicon.json
var lexiconJSON []byte

// Marks phonikud recognises: niqqud, ole, masora, the prefix bar, the geresh.
const markClass = `\x{05ab}\x{05af}\x{05b0}-\x{05c7}|'`

var (
	// Each character as group 1, the marks stacked on it as group 2.
	clusterPattern = regexp.MustCompile(`(?s)(.)([` + markClass + `]*)`)
	markPattern    = regexp.MustCompile(`[` + markClass + `]`)

	// lexicon pins the niqqud of words the diacritizer gets wrong — the intended fix
	// for every future one, baked in and changed by redeploying. Overriding here and
	// not at the IPA level keeps every later stage on the normal path.
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

// NormalizeNiqqud repairs a diacritizer artifact that counts one vowel twice.
//
// phonikud reads ו+dagesh as shuruk /u/ and ו+holam as holam male /o/, so the vav
// carries the vowel. When the diacritizer also marks the consonant before it, the
// vowel is emitted twice: ערוגה -> ʔaʁuuɡa, אורז -> ʔooʁez. The vav's mark is the
// correct one, so the redundant one is dropped — restoring the standard spelling.
func NormalizeNiqqud(vocalized string) string {
	clusters := clusterPattern.FindAllStringSubmatch(vocalized, -1)

	for i := range len(clusters) - 1 {
		cur, next := clusters[i], clusters[i+1]
		if next[1] != "ו" || cur[1] == "ו" {
			continue
		}
		if runes := []rune(cur[1]); len(runes) != 1 || !isHebrewLetter(runes[0]) {
			continue
		}
		// Does the vav carry its own vowel — shuruk or holam male?
		if !strings.ContainsAny(next[2], string([]rune{dagesh, holam, vavHolam})) {
			continue
		}
		marks := strings.Replace(cur[2], string(qubuts), "", 1)
		if marks == cur[2] {
			marks = strings.Replace(cur[2], string(holam), "", 1)
		}
		clusters[i][2] = marks
	}

	var b strings.Builder
	for _, c := range clusters {
		b.WriteString(c[1])
		b.WriteString(c[2])
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

// DoubledVowel reports two identical adjacent vowels, which real Hebrew essentially
// never produces — the hedge matters, since the corpus test pins three legitimate
// hits. The detector for the bug NormalizeNiqqud repairs: run it over new vocabulary.
func DoubledVowel(ipa string) bool {
	var prev rune
	for _, r := range ipa {
		if r == prev && rg.IsVowel(r) {
			return true
		}
		prev = r
	}
	return false
}
