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

// lexicon pins the niqqud of words the diacritizer gets wrong — the fix for every future
// one, baked in. Overriding here and not at the IPA level keeps later stages normal.
var lexicon = map[string]string{
	"חכמה": "חׇכְמָה",
	"שוק":  "שׁוּק",
	"גג":   "גַּג",
	"בית":  "בַּיִת",
}

// isMark reports the marks phonikud recognises: niqqud, ole, masora, prefix bar, geresh.
func isMark(r rune) bool {
	return r >= '\u05b0' && r <= '\u05c7' || strings.ContainsRune("\u05ab\u05af|'", r)
}

func isHebrewLetter(r rune) bool { return r >= 'א' && r <= 'ת' }

func carriesVowel(r rune) bool { return r == dagesh || r == holam || r == vavHolam }

// NormalizeNiqqud repairs a diacritizer artifact that counts one vowel twice: phonikud
// reads ו+dagesh as /u/ and ו+holam as /o/, so marking the consonant before the vav too
// emits the vowel twice (ערוגה -> ʔaʁuuɡa). The vav's mark is the correct one, so the
// redundant one is dropped.
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

// DoubledVowel detects the bug NormalizeNiqqud repairs: two identical adjacent vowels,
// which real Hebrew essentially never produces — though the corpus pins three that do.
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
