// Package heb renders RG for humans: vocalized Hebrew, the same stripped, and hyphenated
// Latin. It works from the niqqud, not the IPA, so it needs no IPA-to-Hebrew generator.
package heb

import (
	"slices"
	"strings"

	"github.com/yardenshoham/rg-language/pkg/rg"
)

// Escapes, not literals: a combining mark stacks on its own quote and cannot be
// proofread. phonikudStress is the diacritizer's "ole", dropped before rendering.
const (
	shva, hatafSegol, hatafPatah, hatafQamats = "\u05b0", "\u05b1", "\u05b2", "\u05b3"
	hiriq, tsere, segol, patah, qamats        = "\u05b4", "\u05b5", "\u05b6", "\u05b7", "\u05b8"
	holam, holamHaser, qubuts, dagesh, meteg  = "\u05b9", "\u05ba", "\u05bb", "\u05bc", "\u05bd"
	qamatsQatan, phonikudStress               = "\u05c7", "\u05ab"
	yod, vav                                  = "\u05d9", "\u05d5"
)

// vowels maps a vowel mark to the spelling the copy in the inserted רג syllable uses.
// Written RG is not standardized — this is the project's convention.
var vowels = map[string]string{
	hiriq: hiriq, tsere: tsere, segol: segol, hatafSegol: segol,
	patah: patah, qamats: qamats, hatafPatah: patah,
	holam: vav + holam, holamHaser: vav + holam, qamatsQatan: vav + holam, hatafQamats: vav + holam,
	qubuts: vav + dagesh,
}

// isMark reports a Hebrew point, accent or cantillation mark; letters start at U+05D0.
func isMark(r rune) bool { return r >= 0x0591 && r <= 0x05c7 }

// StripMarks removes every niqqud mark, which is how people actually write RG.
func StripMarks(text string) string {
	return string(slices.DeleteFunc([]rune(text), isMark))
}

type unit struct{ letter, marks, copied string }

func units(text string) []unit {
	var us []unit
	for _, r := range text {
		if n := len(us) - 1; isMark(r) && n >= 0 {
			us[n].marks += string(r)
			if c, ok := vowels[string(r)]; ok && us[n].copied == "" {
				us[n].copied = c
			}
			continue
		}
		us = append(us, unit{letter: string(r)})
	}
	return us
}

func RG(vocalized string) string {
	var b strings.Builder
	for _, s := range RGSegments(vocalized) {
		b.WriteString(s.Text)
	}
	return b.String()
}

// RGSegments is RG split into original and inserted runs, for highlighting.
func RGSegments(vocalized string) []rg.Segment {
	text := strings.NewReplacer("|", "", phonikudStress, "").Replace(vocalized)
	us := units(text)

	var segs []rg.Segment
	orig := ""
	for i := 0; i < len(us); i++ {
		cur := us[i]
		copied, mater := cur.copied, ""
		switch {
		case cur.letter == vav && strings.ContainsAny(cur.marks, holam+holamHaser):
			copied = vav + holam // holam male: the vav carries it
		case cur.letter == vav && strings.Contains(cur.marks, dagesh) && copied == "" &&
			i > 0 && us[i-1].copied == "":
			copied = vav + dagesh // shuruk
		// A bare yod after i/e is a mater — unless another yod follows, which
		// makes that one consonantal (היי).
		case (copied == hiriq || copied == tsere || copied == segol) && i+1 < len(us) &&
			us[i+1].letter == yod && us[i+1].marks == "" && (i+2 >= len(us) || us[i+2].letter != yod):
			mater, i = yod, i+1 // the mater is consumed with this unit
		case copied == "" && strings.Contains(cur.marks, shva) && strings.Contains(cur.marks, meteg):
			copied = segol // vocal shva
		}

		orig += cur.letter + strings.ReplaceAll(cur.marks, meteg, "") + mater
		if copied != "" {
			// The copy keeps the mater yod, which makes the i unambiguous (פירגיצרגה).
			if copied == hiriq && mater != "" {
				copied += yod
			}
			segs = append(segs, rg.Segment{Text: orig}, rg.Segment{Text: "ר" + shva + "ג" + copied, Inserted: true})
			orig = ""
		}
	}
	if orig != "" {
		segs = append(segs, rg.Segment{Text: orig})
	}
	return segs
}

var latin = strings.NewReplacer("ʃ", "sh", "χ", "kh", "ʁ", "r", "ɡ", "g", "ʔ", "", "ʒ", "zh", "j", "y")

type Syllable struct {
	Text               string
	Stressed, Inserted bool
}

// Syllables splits IPA into per-word syllables. Maximal onset, which makes each
// inserted /ʁɡ/ an onset — so an inserted syllable is exactly one starting ʁɡ.
func Syllables(ipa string) [][]Syllable {
	words := make([][]Syllable, 0, strings.Count(ipa, " ")+1)
	for word := range strings.FieldsSeq(ipa) {
		syllables := make([]Syllable, 0, len(word))
		pending, stressed := "", false
		for _, ch := range word {
			switch {
			case string(ch) == rg.Stress:
				stressed = true
			case rg.IsVowel(ch):
				syllables = append(syllables, Syllable{Text: latin.Replace(pending) + string(ch),
					Stressed: stressed, Inserted: strings.HasPrefix(pending, rg.Insert)})
				pending, stressed = "", false
			default:
				pending += string(ch)
			}
		}
		if pending != "" { // word-final consonants
			if len(syllables) == 0 {
				syllables = append(syllables, Syllable{Inserted: strings.HasPrefix(pending, rg.Insert)})
			}
			syllables[len(syllables)-1].Text += latin.Replace(pending)
		}
		words = append(words, syllables)
	}
	return words
}

// Latin renders IPA as hyphenated syllables, e.g. sha-rga-lo-rgom. Raw IPA is
// never shown to users: it was tried and non-phoneticians cannot read it.
func Latin(ipa string) string {
	words := make([]string, 0, 8)
	for _, w := range Syllables(ipa) {
		parts := make([]string, 0, len(w))
		for _, s := range w {
			parts = append(parts, s.Text)
		}
		words = append(words, strings.Join(parts, "-"))
	}
	return strings.Join(words, " ")
}
