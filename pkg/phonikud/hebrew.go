package phonikud

import "strings"

// The finite-state transducer itself, ported from phonikud's hebrew.py.
//
// It holds no state beyond a three-letter window — previous, current, next —
// which is what makes Hebrew's special cases tractable as a flat chain of rules.
// Several of those rules look arbitrary. They are not: each encodes a real
// Hebrew edge case that a native speaker validated. Do not tidy them.
//
// Reference:
//   - https://en.wikipedia.org/wiki/Help:IPA/Hebrew
//   - https://hebrew-academy.org.il/2020/08/11/איך-הוגים-את-השווא-הנע
//   - https://hebrew-academy.org.il/2022/03/03/מלעיל-ומלרע-על-ההטעמה-בעברית

func phonemizeHebrew(letters []letter) []string {
	var phonemes []string
	for i := 0; i < len(letters); {
		var prev, next *letter
		if i > 0 {
			prev = &letters[i-1]
		}
		if i+1 < len(letters) {
			next = &letters[i+1]
		}
		next2, skipOffset := letterToPhonemes(letters[i], prev, next)
		phonemes = append(phonemes, next2...)
		i += skipOffset + 1
	}
	return phonemes
}

// handleYud reports whether a yod is a mater lectionis rather than a consonant,
// in which case it contributes no sound of its own.
func handleYud(cur letter, prev, next *letter) bool {
	return next != nil &&
		cur.diac == "" && // yod without diacritics
		prev != nil && // in the middle of the word
		prev.char+prev.diac != "א\u05b5" && // previous is not אֵ (alef with tsere)
		// and the next vav does not have a meaning of its own
		(next.char != "ו" || next.diac == "" || strings.ContainsRune(next.diac, shva))
}

// handleVav resolves the vav, which is a consonant, two different vowels, or
// silent, depending entirely on its marks and its neighbours.
func handleVav(cur letter, prev, next *letter) (phonemes []string, skipConsonants, skipDiacritics bool, skipOffset int) {
	if prev != nil && strings.ContainsRune(prev.diac, shva) && strings.ContainsRune(cur.diac, holam) {
		return []string{"vo"}, true, true, 0
	}

	if next != nil && next.char == "ו" {
		diac := cur.diac + next.diac
		switch {
		case strings.ContainsRune(diac, holam):
			return []string{"vo"}, true, true, 1
		case cur.diac == next.diac:
			return []string{"vu"}, true, true, 1
		case strings.ContainsRune(cur.diac, hirik):
			return []string{"vi"}, true, true, 0
		case strings.ContainsRune(cur.diac, shva) && next.diac == "":
			return []string{"v"}, true, true, 0
		case strings.ContainsRune(cur.diac, kamatz) || strings.ContainsRune(cur.diac, patah):
			return []string{"va"}, true, true, 0
		case strings.ContainsRune(cur.diac, tsere) || strings.ContainsRune(cur.diac, segol):
			return []string{"ve"}, true, true, 0
		}
		return nil, false, false, 0
	}

	// A single vav.
	switch {
	case strings.ContainsRune(cur.diac, patah) || strings.ContainsRune(cur.diac, kamatz):
		return []string{"va"}, true, true, 0
	case strings.ContainsRune(cur.diac, tsere) || strings.ContainsRune(cur.diac, segol):
		return []string{"ve"}, true, true, 0
	case strings.ContainsRune(cur.diac, holam):
		return []string{"o"}, true, true, 0
	case strings.ContainsRune(cur.diac, kubuts) || strings.ContainsRune(cur.diac, dagesh):
		return []string{"u"}, true, true, 0
	case strings.ContainsRune(cur.diac, shva) && prev == nil:
		return []string{"ve"}, true, true, 0
	case strings.ContainsRune(cur.diac, hirik):
		return []string{"vi"}, true, true, 0
	case next != nil && cur.diac == "":
		return nil, true, true, 0
	}
	return []string{"v"}, true, true, 0
}

func letterToPhonemes(cur letter, prev, next *letter) ([]string, int) {
	var out []string
	skipConsonants, skipDiacritics := false, false
	skipOffset := 0

	switch {
	case strings.ContainsRune(cur.allDiac, nikudHaserDiacritic):
		skipConsonants, skipDiacritics = true, true

	case cur.char == "א" && cur.diac == "" && prev != nil:
		if next != nil && next.char != "ו" {
			skipConsonants = true
		}

	case cur.char == "י" && handleYud(cur, prev, next):
		skipConsonants = true

	case cur.char == "ש" && strings.ContainsRune(cur.diac, sin):
		if next != nil && next.char == "ש" && next.diac == "" &&
			(strings.ContainsRune(cur.diac, patah) || strings.ContainsRune(cur.diac, kamatz)) {
			out = append(out, "sa") // יששכר
			skipConsonants, skipDiacritics = true, true
			skipOffset++
			break
		}
		out = append(out, "s")
		skipConsonants = true

	// Shin without nikud after a sin is itself a sin.
	case cur.char == "ש" && cur.diac == "" && prev != nil && strings.ContainsRune(prev.diac, sin):
		out = append(out, "s")
		skipConsonants = true

	case next == nil && cur.char == "ח" && strings.ContainsRune(cur.diac, patah):
		out = append(out, "ax") // final het gnuva
		skipConsonants, skipDiacritics = true, true

	case next == nil && cur.char == "ה" && strings.ContainsRune(cur.diac, patah):
		out = append(out, "ah") // final he gnuva
		skipConsonants, skipDiacritics = true, true

	case next == nil && cur.char == "ע" && strings.ContainsRune(cur.diac, patah):
		out = append(out, "a") // final ayin gnuva
		skipConsonants, skipDiacritics = true, true
	}

	geresh, hasGeresh := gereshPhonemes[cur.char]
	withDagesh, hasDagesh := lettersPhonemes[cur.char+string(dagesh)]
	switch {
	case strings.ContainsRune(cur.diac, '\'') && hasGeresh:
		out = append(out, geresh)
		skipConsonants = true
		if cur.char == "ת" {
			skipDiacritics = true
		}

	case strings.ContainsRune(cur.diac, dagesh) && hasDagesh:
		out = append(out, withDagesh)
		skipConsonants = true

	case cur.char == "ו" && !strings.ContainsRune(cur.allDiac, nikudHaserDiacritic):
		vavPhonemes, vavSkipConsonants, vavSkipDiacritics, vavSkipOffset := handleVav(cur, prev, next)
		out = append(out, vavPhonemes...)
		skipConsonants, skipDiacritics = vavSkipConsonants, vavSkipDiacritics
		skipOffset += vavSkipOffset
	}

	if !skipConsonants {
		out = append(out, lettersPhonemes[cur.char])
	}

	// Kamatz before a hataf kamatz is a kamatz katan, so it sounds /o/.
	if strings.ContainsRune(cur.diac, kamatz) && next != nil && strings.ContainsRune(next.diac, hatafKamatz) {
		out = append(out, "o")
		skipDiacritics = true
	}

	switch {
	case !skipDiacritics:
		for _, mark := range cur.allDiac {
			out = append(out, nikudPhonemes[mark])
		}
	case strings.ContainsRune(cur.allDiac, hatamaDiacritic):
		// Even a silent letter keeps its stress.
		out = append(out, stressPhoneme)
	}

	return keepPhonemes(sortStress(out)), skipOffset
}

// sortStress moves the stress mark to just before the vowel, which is where TTS
// systems expect it; linguistics would put it at the start of the syllable.
func sortStress(phonemes []string) []string {
	joined := strings.Join(phonemes, "")
	if !strings.Contains(joined, stressPhoneme) || !strings.ContainsAny(joined, "aeiou") {
		return phonemes
	}

	kept := make([]string, 0, len(phonemes))
	for _, p := range phonemes {
		if p != stressPhoneme {
			kept = append(kept, p)
		}
	}
	for i, p := range kept {
		if at := strings.IndexAny(p, "aeiou"); at >= 0 {
			kept[i] = p[:at] + stressPhoneme + p[at:]
			break
		}
	}
	return kept
}

// keepPhonemes drops anything that is not made entirely of output characters,
// which is how Hebrew letters, digits and quotes leave the stream.
func keepPhonemes(phonemes []string) []string {
	kept := phonemes[:0]
	for _, p := range phonemes {
		if p == "" {
			continue
		}
		if strings.ContainsFunc(p, func(r rune) bool { return !setPhonemes[r] }) {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}
