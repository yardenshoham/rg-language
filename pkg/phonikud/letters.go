package phonikud

import (
	"regexp"
	"slices"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var (
	// A letter and its stacked marks, sorted so one vowel has only one spelling.
	sortDiacriticsRe = regexp.MustCompile(`\p{L}\p{M}+`)

	// deduplicate folds the Hebrew-specific punctuation onto its ASCII twin.
	deduplicate = strings.NewReplacer(
		"״", `"`, // gershayim
		"׳", "'", // geresh
		"\u05be", "-", // maqaf
	)

	// Letters and their marks; the geresh and prefix bar ride with the letter they follow.
	lettersRe = regexp.MustCompile(`(\p{L})([\p{M}'|]*)`)

	hePattern = regexp.MustCompile(hePatternText) //nolint:gocritic // the sheva-to-tav range spans marks and letters on purpose
)

// normalize decomposes, sorts each letter's marks and folds punctuation to ASCII.
func normalize(text string) string {
	sorted := sortDiacriticsRe.ReplaceAllStringFunc(norm.NFD.String(text), func(match string) string {
		runes := []rune(match)
		slices.Sort(runes[1:])
		return string(runes)
	})
	return deduplicate.Replace(sorted)
}

// letter is one letter with its stacked marks. allDiac keeps every mark; diac
// drops phonikud's three extra ones, so "has a vowel?" is not fooled by stress.
type letter struct {
	char    string
	allDiac string
	diac    string
}

// plainDiacs drops phonikud's three extra marks, leaving the vowel.
var plainDiacs = strings.NewReplacer(
	string(hatamaDiacritic), "", string(prefixDiacritic), "", string(vocalShvaDiacritic), "")

func newLetter(char, diac string) letter {
	all := normalize(diac)
	return letter{char: normalize(char), allDiac: all, diac: plainDiacs.Replace(all)}
}

func (l letter) String() string { return l.char + l.allDiac }

func getLetters(word string) []letter {
	matches := lettersRe.FindAllStringSubmatch(word, -1)
	letters := make([]letter, 0, len(matches))
	for _, m := range matches {
		letters = append(letters, newLetter(m[1], m[2]))
	}
	return letters
}

// hasVowelDiacs reports a mark that counts as a vowel: a plain shva does not, a meteg does.
func hasVowelDiacs(s string) bool {
	// Shuruk: the vav is itself the vowel.
	return s == "ו\u05bc" || strings.ContainsFunc(s, func(r rune) bool {
		return (r >= hatafSegol && r <= kubuts) || r == kamatzKatan || r == vocalShvaDiacritic
	})
}

// getSyllables splits a word into syllables, accurately only for the last one.
func getSyllables(word string) []string {
	letters := getLetters(word)
	isVav := func(i int) bool { return i < len(letters) && letters[i].char == "ו" }

	var syllables []string
	var cur string
	vowelState := false
	for i := 0; i < len(letters); {
		l := letters[i].String()
		hasVowel := hasVowelDiacs(l) || (i == 0 && strings.ContainsRune(l, shva))
		if hasVowel && vowelState {
			syllables = append(syllables, cur)
			cur = ""
		}
		cur += l
		vowelState = vowelState || hasVowel
		i++

		// A vav always starts its own syllable, so look ahead for one or two.
		switch {
		case isVav(i+1) && isVav(i+2):
			syllables = append(syllables, cur+letters[i].String())
			cur = letters[i+1].String() + letters[i+2].String()
			i += 3
			vowelState = true
		case isVav(i+1) && letters[i+1].diac != "":
			syllables = append(syllables, cur)
			cur = ""
			vowelState = false
		}
	}
	if cur != "" {
		syllables = append(syllables, cur)
	}
	return syllables
}

// addMilraHatama stresses the last syllable, Hebrew's default (milra).
func addMilraHatama(word string) string {
	syllables := getSyllables(word)
	if len(syllables) == 0 {
		return word
	}
	last := len(syllables) - 1
	first := lettersRe.FindStringIndex(syllables[last]) // the syllable's first letter and marks
	if first == nil {
		return word
	}
	syllables[last] = syllables[last][:first[1]] + string(hatamaDiacritic) + syllables[last][first[1]:]
	return strings.Join(syllables, "")
}

// sortHatama moves stress off a letter carrying masora, which is not pronounced.
func sortHatama(letters []letter) []letter {
	for i := range len(letters) - 1 {
		d := letters[i].allDiac
		if !strings.ContainsRune(d, hatamaDiacritic) || !strings.ContainsRune(d, nikudHaserDiacritic) {
			continue
		}
		letters[i].allDiac = strings.Replace(d, string(hatamaDiacritic), "", 1)
		letters[i+1].allDiac += string(hatamaDiacritic)
	}
	return letters
}
