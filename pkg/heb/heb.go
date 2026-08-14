// Package heb renders RG output for humans: Hebrew letters with niqqud, the
// same stripped of its marks, and hyphenated Latin syllables.
//
// RG runs on the vocalized Hebrew rather than converting IPA back to letters.
// The niqqud already marks every vowel, so the same "insert after each vowel"
// rule applies directly to the text — which avoids writing an IPA-to-Hebrew
// generator, with its matres lectionis, final letter forms and silent א/ה/ע.
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

	// phonikudStress is the "ole" mark the diacritizer uses for stress. It is
	// not part of the spelling, so it is dropped before rendering.
	phonikudStress = "֫"

	yod = "י"
	vav = "ו"
)

// vowelSound maps a vowel mark to the sound it makes.
var vowelSound = map[string]string{
	hiriq: "i", tsere: "e", segol: "e", patah: "a", qamats: "a",
	holam: "o", holamHaser: "o", qubuts: "u", qamatsQatan: "o",
	hatafSegol: "e", hatafPatah: "a", hatafQamats: "o",
}

// copyMark is how the duplicated vowel is spelled in the inserted רג syllable.
// The copy reuses the original mark so it looks like the vowel it came from,
// except that hatafs normalize to their plain form and o/u always take their
// vav spelling. Written RG is not standardized; this is the project's own
// convention.
var copyMark = map[string]string{
	hatafSegol: segol, hatafPatah: patah, hatafQamats: vav + holam,
	holam: vav + holam, holamHaser: vav + holam, qamatsQatan: vav + holam,
	qubuts: vav + dagesh,
}

var fallbackSpelling = map[string]string{
	"a": patah, "e": segol, "i": hiriq, "o": vav + holam, "u": vav + dagesh,
}

// isMark reports whether r is a Hebrew point, accent or cantillation mark.
// The Hebrew letters start at U+05D0, above this range, so nothing overlaps.
func isMark(r rune) bool { return r >= 0x0591 && r <= 0x05c7 }

// StripMarks removes every niqqud mark, turning the vocalized rendering into
// the unvocalized one — which is how people actually write RG.
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

func (u unit) hasVowel() bool {
	for _, m := range u.marks {
		if _, ok := vowelSound[string(m)]; ok {
			return true
		}
	}
	return false
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

// RG renders vocalized Hebrew as RG, with niqqud. This is the authoritative
// pronunciation guide.
func RG(vocalized string) string {
	var b strings.Builder
	for _, s := range RGSegments(vocalized) {
		b.WriteString(s.Text)
	}
	return b.String()
}

// RGSegments is RG split into runs of original and inserted text, so the UI can
// highlight exactly what the rule added.
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

	at := func(i int) *unit {
		if i < len(us) {
			return &us[i]
		}
		return nil
	}

	for i := 0; i < len(us); {
		cur, next, next2 := us[i], at(i+1), at(i+2)

		var sound, mark, mater string
		var vowelMarks []string
		for _, m := range cur.marks {
			if _, ok := vowelSound[string(m)]; ok {
				vowelMarks = append(vowelMarks, string(m))
			}
		}

		switch {
		case cur.letter == vav && (strings.Contains(cur.marks, holam) || strings.Contains(cur.marks, holamHaser)):
			sound = "o" // holam male: the vav carries it
		case cur.letter == vav && strings.Contains(cur.marks, dagesh) && len(vowelMarks) == 0 &&
			i > 0 && !us[i-1].hasVowel():
			sound = "u" // shuruk
		case len(vowelMarks) > 0:
			mark = vowelMarks[0]
			sound = vowelSound[mark]
			// A bare yod after i/e is a mater and belongs to the vowel — unless
			// another yod follows it, in which case that one is consonantal (היי).
			if (sound == "i" || sound == "e") && next != nil && next.letter == yod &&
				next.marks == "" && (next2 == nil || next2.letter != yod) {
				mater = yod
			}
		case strings.Contains(cur.marks, shva) && strings.Contains(cur.marks, meteg):
			sound = "e" // vocal shva
		}

		add(cur.letter+strings.ReplaceAll(cur.marks, meteg, "")+mater, false)
		if sound != "" {
			copied := fallbackSpelling[sound]
			if mark != "" {
				copied = mark
				if c, ok := copyMark[mark]; ok {
					copied = c
				}
			}
			if sound == "i" && mater != "" {
				copied += yod
			}
			add("ר"+shva+"ג"+copied, true)
		}

		if mater != "" {
			i += 2
		} else {
			i++
		}
	}
	return segs
}

// latin transliterates the IPA symbols that a non-phonetician cannot read.
var latin = map[rune]string{
	'ʃ': "sh", 'χ': "kh", 'ʁ': "r", 'ɡ': "g", 'ʔ': "", 'ʒ': "zh", 'j': "y",
}

// Syllable is one Latin syllable of the readable fallback rendering.
type Syllable struct {
	Text     string
	Stressed bool
	Inserted bool
}

// Syllables splits IPA into per-word lists of syllables.
//
// Maximal onset, which conveniently makes each inserted /ʁɡ/ the onset of the
// duplicated vowel — so an inserted syllable is exactly one starting with ʁɡ.
func Syllables(ipa string) [][]Syllable {
	var words [][]Syllable
	for word := range strings.FieldsSeq(ipa) {
		type chunk struct {
			text     string
			stressed bool
		}
		var chunks []chunk
		pending, stressed := "", false
		for _, ch := range word {
			switch {
			case string(ch) == rg.Stress:
				stressed = true
			case strings.ContainsRune("aeiou", ch):
				chunks = append(chunks, chunk{pending + string(ch), stressed})
				pending, stressed = "", false
			default:
				pending += string(ch)
			}
		}
		if pending != "" { // word-final consonants
			if len(chunks) > 0 {
				chunks[len(chunks)-1].text += pending
			} else {
				chunks = append(chunks, chunk{pending, false})
			}
		}

		syllables := make([]Syllable, 0, len(chunks))
		for _, c := range chunks {
			var b strings.Builder
			for _, r := range c.text {
				if l, ok := latin[r]; ok {
					b.WriteString(l)
					continue
				}
				b.WriteRune(r)
			}
			syllables = append(syllables, Syllable{
				Text:     b.String(),
				Stressed: c.stressed,
				Inserted: strings.HasPrefix(c.text, rg.Insert),
			})
		}
		words = append(words, syllables)
	}
	return words
}

// Latin renders IPA as readable hyphenated syllables, e.g. sha-rga-lo-rgom.
// Raw IPA is never shown to users: it was tried and it is unreadable to
// non-phoneticians.
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
