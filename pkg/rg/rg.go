// Package rg applies the Resh-Gimel transform to IPA phonemes.
//
// No syllabification is needed: the inserted material always lands right after a
// vowel, so the syllable boundary is invisible in the output — sha|lom and shal|om
// both give shargalorgom. The rule reduces to "after every vowel insert /ʁɡ/ plus
// a copy of it", which is a per-letter yes/no.
package rg

import "strings"

// Stress is U+02C8, Insert the /ʁɡ/ cluster. Both are multi-byte, so this works
// on runes.
const (
	Stress = "ˈ"
	Insert = "ʁɡ"
)

// StressMode decides which copy of a duplicated vowel keeps the stress.
type StressMode string

const (
	// StressFirst keeps the stress on the original and the רג copy unstressed. The
	// default: it won 5 of 6 blind A/B picks, plus four notes saying a stressed
	// copy pushed the רג ("that part should almost be silent").
	StressFirst StressMode = "first"
	// StressSecond stresses the copy. Kept because stress may be word-dependent —
	// גנן was picked both ways across rounds.
	StressSecond StressMode = "second"
	// StressBoth stresses both copies.
	StressBoth StressMode = "both"
)

// Segment is a run of transformed text; Inserted marks what the rule added, so the
// UI can highlight it. Only package heb builds these — raw IPA is never shown to
// users, so Transform returns a plain string.
type Segment struct {
	Text     string
	Inserted bool
}

// IsVowel reports whether r is one of the five IPA vowels the rule fires on.
func IsVowel(r rune) bool { return r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u' }

// Transform applies the RG rule to an IPA string.
func Transform(ipa string, mode StressMode) string {
	firstStress, secondStress := "", ""
	if mode == StressFirst || mode == StressBoth {
		firstStress = Stress
	}
	if mode == StressSecond || mode == StressBoth {
		secondStress = Stress
	}

	runes := []rune(ipa)
	var b strings.Builder
	for i := 0; i < len(runes); i++ {
		// The mark precedes its vowel, so the two move together and mode decides
		// which copy keeps it.
		if string(runes[i]) == Stress && i+1 < len(runes) && IsVowel(runes[i+1]) {
			i++
			v := string(runes[i])
			b.WriteString(firstStress + v + Insert + secondStress + v)
			continue
		}
		v := string(runes[i])
		b.WriteString(v)
		if IsVowel(runes[i]) {
			b.WriteString(Insert + v)
		}
	}
	return b.String()
}
