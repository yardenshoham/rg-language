// Package heb renders RG for humans: Hebrew with niqqud, the same stripped, and
// hyphenated Latin syllables.
//
// It runs on the vocalized Hebrew rather than converting IPA back to letters. The
// niqqud already marks every vowel, so the same rule applies directly to the text,
// which avoids an IPA-to-Hebrew generator and its matres lectionis, final forms
// and silent א/ה/ע.
package heb

import (
	"strings"

	"github.com/yardenshoham/rg-language/pkg/rg"
)

// Hebrew marks, by their Unicode names.
const (
	shva        = "ְ"
	hatafSegol  = "ֱ"
	hatafPatah  = "ֲ"
	hatafQamats = "ֳ"
	hiriq       = "ִ"
	tsere       = "ֵ"
	segol       = "ֶ"
	patah       = "ַ"
	qamats      = "ָ"
	holam       = "ֹ"
	holamHaser  = "ֺ"
	qubuts      = "ֻ"
	dagesh      = "ּ"
	meteg       = "ֽ"
	qamatsQatan = "ׇ"

	// phonikudStress is the diacritizer's "ole" stress mark, not part of the
	// spelling, so it is dropped before rendering.
	phonikudStress = "֫"

	yod = "י"
	vav = "ו"
)

// vowels maps a vowel mark to its sound and to the spelling the copy in the
// inserted רג syllable uses. Most are reused verbatim so the copy looks like the
// vowel it came from; hatafs take their plain form and o/u their vav spelling.
// Written RG is not standardized — this is the project's own convention.
var vowels = map[string]struct{ sound, copied string }{
	hiriq: {"i", hiriq}, tsere: {"e", tsere}, segol: {"e", segol},
	patah: {"a", patah}, qamats: {"a", qamats},
	holam: {"o", vav + holam}, holamHaser: {"o", vav + holam},
	qamatsQatan: {"o", vav + holam}, qubuts: {"u", vav + dagesh},
	hatafSegol: {"e", segol}, hatafPatah: {"a", patah},
	hatafQamats: {"o", vav + holam},
}

// isMark reports whether r is a Hebrew point, accent or cantillation mark. The
// letters start at U+05D0, above this range, so nothing overlaps.
func isMark(r rune) bool { return r >= 0x0591 && r <= 0x05c7 }

// StripMarks removes every niqqud mark, which is how people actually write RG.
func StripMarks(text string) string {
	return strings.Map(func(r rune) rune {
		if isMark(r) {
			return -1
		}
		return r
	}, text)
}

type unit struct {
	letter string
	marks  string
}

// vowel returns the first vowel mark on u, or "" when it carries none.
func (u unit) vowel() string {
	for _, m := range u.marks {
		if _, ok := vowels[string(m)]; ok {
			return string(m)
		}
	}
	return ""
}

// units splits text into (letter, marks) pairs.
func units(text string) []unit {
	var us []unit
	for _, r := range text {
		if isMark(r) && len(us) > 0 {
			us[len(us)-1].marks += string(r)
			continue
		}
		us = append(us, unit{letter: string(r)})
	}
	return us
}

// RG renders vocalized Hebrew as RG, with niqqud: the pronunciation guide.
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
	add := func(text string, inserted bool) {
		if n := len(segs); n > 0 && segs[n-1].Inserted == inserted {
			segs[n-1].Text += text
			return
		}
		segs = append(segs, rg.Segment{Text: text, Inserted: inserted})
	}

	for i := 0; i < len(us); i++ {
		cur := us[i]
		vowel := cur.vowel()

		var sound, copied, mater string
		switch {
		case cur.letter == vav && (strings.Contains(cur.marks, holam) || strings.Contains(cur.marks, holamHaser)):
			sound, copied = "o", vav+holam // holam male: the vav carries it
		case cur.letter == vav && strings.Contains(cur.marks, dagesh) && vowel == "" &&
			i > 0 && us[i-1].vowel() == "":
			sound, copied = "u", vav+dagesh // shuruk
		case vowel != "":
			sound, copied = vowels[vowel].sound, vowels[vowel].copied
			// A bare yod after i/e is a mater — unless another yod follows, which
			// makes that one consonantal (היי).
			if (sound == "i" || sound == "e") && i+1 < len(us) && us[i+1].letter == yod &&
				us[i+1].marks == "" && (i+2 >= len(us) || us[i+2].letter != yod) {
				mater, i = yod, i+1 // the mater is consumed with this unit
			}
		case strings.Contains(cur.marks, shva) && strings.Contains(cur.marks, meteg):
			sound, copied = "e", segol // vocal shva
		}

		add(cur.letter+strings.ReplaceAll(cur.marks, meteg, "")+mater, false)
		if sound != "" {
			if sound == "i" && mater != "" {
				copied += yod
			}
			add("ר"+shva+"ג"+copied, true)
		}
	}
	return segs
}

// latin transliterates the IPA symbols that a non-phonetician cannot read.
var latin = strings.NewReplacer(
	"ʃ", "sh", "χ", "kh", "ʁ", "r", "ɡ", "g", "ʔ", "", "ʒ", "zh", "j", "y",
)

// Syllable is one Latin syllable of the readable fallback rendering.
type Syllable struct {
	Text     string
	Stressed bool
	Inserted bool
}

// Syllables splits IPA into per-word syllables. Maximal onset, which makes each
// inserted /ʁɡ/ an onset — so an inserted syllable is exactly one starting ʁɡ.
func Syllables(ipa string) [][]Syllable {
	var words [][]Syllable
	for word := range strings.FieldsSeq(ipa) {
		syllables := make([]Syllable, 0, len(word))
		pending, stressed := "", false
		for _, ch := range word {
			switch {
			case string(ch) == rg.Stress:
				stressed = true
			case rg.IsVowel(ch):
				// The /ʁɡ/ is in the onset, so pending alone identifies it.
				syllables = append(syllables, Syllable{
					Text:     latin.Replace(pending) + string(ch),
					Stressed: stressed,
					Inserted: strings.HasPrefix(pending, rg.Insert),
				})
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
