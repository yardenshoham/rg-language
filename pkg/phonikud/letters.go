package phonikud

import (
	"regexp"
	"slices"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var (
	// A letter and its stacked marks, so they can be ordered canonically — two
	// spellings of one vowel must not give two results.
	sortDiacriticsRe = regexp.MustCompile(`\p{L}\p{M}+`)

	// deduplicate folds the Hebrew-specific punctuation onto its ASCII twin.
	deduplicate = strings.NewReplacer(
		"״", `"`, // gershayim
		"׳", "'", // geresh
		"\u05be", "-", // maqaf
	)

	// Letters and their marks. The en geresh and prefix bar ride along with the
	// letter they follow.
	lettersRe = regexp.MustCompile(`(\p{L})([\p{M}'|]*)`)

	hePattern = regexp.MustCompile(hePatternText)
)

// normalize decomposes, sorts each letter's marks and folds punctuation to ASCII.
func normalize(text string) string {
	text = norm.NFD.String(text)
	text = sortDiacriticsRe.ReplaceAllStringFunc(text, func(match string) string {
		runes := []rune(match)
		marks := runes[1:]
		slices.Sort(marks)
		return string(runes[0]) + string(marks)
	})
	return deduplicate.Replace(text)
}

// letter is one letter with its stacked marks. allDiac keeps every mark; diac
// drops phonikud's three extra ones, so a rule asking "does this letter have a
// vowel" is not fooled by a stress mark.
type letter struct {
	char    string
	allDiac string
	diac    string
}

func newLetter(char, diac string) letter {
	all := normalize(diac)
	var plain strings.Builder
	for _, r := range all {
		if !isEnhancedDiacritic(r) {
			plain.WriteRune(r)
		}
	}
	return letter{char: normalize(char), allDiac: all, diac: plain.String()}
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

// isVowelDiac reports whether a mark counts as a vowel for syllable splitting: a
// plain shva does not, but meteg (a vocal shva) does.
func isVowelDiac(r rune) bool {
	return (r >= hatafSegol && r <= kubuts) || r == kamatzKatan || r == vocalShvaDiacritic
}

func hasVowelDiacs(s string) bool {
	if s == "ו\u05bc" { // shuruk: the vav itself is the vowel
		return true
	}
	return strings.ContainsFunc(s, isVowelDiac)
}

// getSyllables splits a word into syllables, accurately enough only to find the
// last one — all the stress prediction below needs.
func getSyllables(word string) []string {
	letters := getLetters(word)
	var syllables []string
	var cur string
	vowelState := false

	for i := 0; i < len(letters); {
		l := letters[i]
		hasVowel := hasVowelDiacs(l.String()) ||
			(i == 0 && strings.ContainsRune(l.allDiac, shva))

		// Look ahead for a vav, which always starts its own syllable.
		vav1 := i+2 < len(letters) && letters[i+2].char == "ו"
		vav2 := i+3 < len(letters) && letters[i+3].char == "ו"

		if hasVowel && vowelState {
			syllables = append(syllables, cur)
			cur = l.String()
		} else {
			cur += l.String()
		}
		if hasVowel {
			vowelState = true
		}
		i++

		switch {
		case vav1 && vav2:
			// Two vavs coming: close the current syllable and join both as the next.
			if cur != "" {
				syllables = append(syllables, cur+letters[i].String())
			}
			cur = letters[i+1].String() + letters[i+2].String()
			i += 3
			vowelState = true
		case vav1 && letters[i+1].diac != "":
			// One vav coming: close the syllable now.
			if cur != "" {
				syllables = append(syllables, cur)
				cur = ""
			}
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
	letters := getLetters(syllables[last])
	if len(letters) == 0 {
		return word
	}
	letters[0].allDiac += string(hatamaDiacritic)

	var b strings.Builder
	for _, l := range letters {
		b.WriteString(l.String())
	}
	syllables[last] = b.String()
	return strings.Join(syllables, "")
}

// sortHatama moves stress off a letter carrying masora: it is not pronounced, so
// it cannot carry the stress.
func sortHatama(letters []letter) []letter {
	for i := range len(letters) - 1 {
		diacs := []rune(letters[i].allDiac)
		at := slices.Index(diacs, hatamaDiacritic)
		if at < 0 || !slices.Contains(diacs, nikudHaserDiacritic) {
			continue
		}
		letters[i].allDiac = string(slices.Delete(diacs, at, at+1))
		letters[i+1].allDiac += string(hatamaDiacritic)
	}
	return letters
}
