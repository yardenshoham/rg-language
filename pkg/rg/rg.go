// Package rg applies the Resh-Gimel transform to IPA phonemes: after every vowel,
// insert /ʁɡ/ and a copy of that vowel.
//
// No syllabification is needed: the insert lands right after a vowel, so the syllable
// boundary is invisible — sha|lom and shal|om both give shargalorgom.
package rg

import "strings"

// Stress is U+02C8 and Insert the /ʁɡ/ cluster; both are multi-byte, so work on runes.
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
	StressBoth   StressMode = "both"
)

// Segment is a run of transformed text; Inserted marks what the rule added, for the
// UI to highlight. Built only by package heb: raw IPA is never shown to users.
type Segment struct {
	Text     string
	Inserted bool
}

func IsVowel(r rune) bool { return r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u' }

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
		v, first, second := string(runes[i]), "", ""
		// The mark precedes its vowel, so the two move together.
		if v == Stress && i+1 < len(runes) && IsVowel(runes[i+1]) {
			i++
			v, first, second = string(runes[i]), firstStress, secondStress
		}
		b.WriteString(first + v)
		if IsVowel(runes[i]) {
			b.WriteString(Insert + second + v)
		}
	}
	return b.String()
}
