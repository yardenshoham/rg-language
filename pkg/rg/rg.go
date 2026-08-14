// Package rg applies the Resh-Gimel transform to IPA phonemes.
//
// The transform needs no syllabification at all. Because the inserted material
// always lands immediately after a vowel, the coda/onset boundary is invisible
// in the output string:
//
//	sha|lom  -> sh a [rga] | l o [rgo] m   = shargalorgom
//	shal|om  -> sh a [rga] l | o [rgo] m   = shargalorgom   (identical)
//
// So the rule reduces to: after every vowel, insert /ʁɡ/ plus a copy of that
// vowel. Getting the vowels right is a per-letter yes/no; where syllables divide
// never has to be decided.
package rg

// Stress is U+02C8, the IPA primary stress mark, and Insert is the /ʁɡ/ cluster
// the rule adds. Both are multi-byte in UTF-8, so everything here works on runes.
const (
	Stress = "ˈ"
	Insert = "ʁɡ"
)

// StressMode decides which copy of a duplicated vowel keeps the stress.
type StressMode string

const (
	// StressFirst leaves the stress on the original vowel and keeps the inserted
	// רג copy unstressed. It is the default: in blind listening it won 5 of 6
	// A/B picks, with four more notes complaining that stressing the copy pushed
	// the רג ("that part should almost be silent").
	StressFirst StressMode = "first"
	// StressSecond stresses the copy instead. Kept because stress may be mildly
	// word-dependent — גנן was picked both ways across rounds.
	StressSecond StressMode = "second"
	// StressBoth stresses both copies.
	StressBoth StressMode = "both"
)

// Segment is a run of transformed text. Inserted marks the runs the rule added,
// so the UI can highlight exactly what changed.
type Segment struct {
	Text     string
	Inserted bool
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// Transform applies the RG rule to an IPA string.
func Transform(ipa string, mode StressMode) string {
	var b []byte
	for _, s := range TransformSegments(ipa, mode) {
		b = append(b, s.Text...)
	}
	return string(b)
}

// TransformSegments applies the RG rule and returns the result split into runs
// of original and inserted text.
func TransformSegments(ipa string, mode StressMode) []Segment {
	firstStress, secondStress := "", ""
	if mode == StressFirst || mode == StressBoth {
		firstStress = Stress
	}
	if mode == StressSecond || mode == StressBoth {
		secondStress = Stress
	}

	runes := []rune(ipa)
	var segs []Segment
	add := func(text string, inserted bool) {
		if n := len(segs); n > 0 && segs[n-1].Inserted == inserted {
			segs[n-1].Text += text
			return
		}
		segs = append(segs, Segment{Text: text, Inserted: inserted})
	}

	for i := 0; i < len(runes); i++ {
		switch {
		case string(runes[i]) == Stress && i+1 < len(runes) && isVowel(runes[i+1]):
			v := string(runes[i+1])
			add(firstStress+v, false)
			add(Insert+secondStress+v, true)
			i++
		case isVowel(runes[i]):
			v := string(runes[i])
			add(v, false)
			add(Insert+v, true)
		default:
			add(string(runes[i]), false)
		}
	}
	return segs
}
