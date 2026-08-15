package pipeline

import (
	"slices"
	"strings"

	"github.com/yardenshoham/rg-language/pkg/rg"
)

const (
	qubuts   = '\u05bb'
	holam    = '\u05b9'
	vavHolam = '\u05ba'
	dagesh   = '\u05bc'
)

// lexicon pins the niqqud of words the diacritizer gets wrong — the intended fix for
// every future one, baked in and changed by redeploying. Overriding here and not at
// the IPA level keeps every later stage on the normal path.
var lexicon = map[string]string{
	"חכמה": "חׇכְמָה",
	"שוק":  "שׁוּק",
	"גג":   "גַּג",
	"בית":  "בַּיִת",
}

// isMark reports the marks phonikud recognises: niqqud, ole, masora, the prefix bar,
// the geresh.
func isMark(r rune) bool {
	return r >= '\u05b0' && r <= '\u05c7' || strings.ContainsRune("\u05ab\u05af|'", r)
}

func isHebrewLetter(r rune) bool { return r >= 'א' && r <= 'ת' }

func carriesVowel(r rune) bool { return r == dagesh || r == holam || r == vavHolam }

// NormalizeNiqqud repairs a diacritizer artifact that counts one vowel twice.
//
// phonikud reads ו+dagesh as shuruk /u/ and ו+holam as holam male /o/, so the vav
// carries the vowel. When the diacritizer also marks the consonant before it, the
// vowel is emitted twice: ערוגה -> ʔaʁuuɡa, אורז -> ʔooʁez. The vav's mark is the
// correct one, so the redundant one is dropped — restoring the standard spelling.
func NormalizeNiqqud(vocalized string) string {
	runes := []rune(vocalized)
	prev := -1 // head of the previous letter-plus-marks cluster
	for i := 0; i < len(runes); i++ {
		head := i
		for i+1 < len(runes) && isMark(runes[i+1]) {
			i++
		}
		if runes[head] == 'ו' && prev >= 0 && runes[prev] != 'ו' && isHebrewLetter(runes[prev]) &&
			slices.ContainsFunc(runes[head+1:i+1], carriesVowel) {
			marks := runes[prev+1 : head]
			for _, redundant := range []rune{qubuts, holam} {
				if at := slices.Index(marks, redundant); at >= 0 {
					marks[at] = -1 // dropped on the way out
					break
				}
			}
		}
		prev = head
	}
	return string(slices.DeleteFunc(runes, func(r rune) bool { return r < 0 }))
}

// ApplyLexicon replaces each word whose bare spelling is pinned in the lexicon.
func ApplyLexicon(vocalized string) string {
	words := strings.Split(vocalized, " ")
	for i, word := range words {
		if pinned, ok := lexicon[string(slices.DeleteFunc([]rune(word), isMark))]; ok {
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
