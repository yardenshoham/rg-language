package phonikud

import (
	"slices"
	"strings"
)

// The transducer, ported from phonikud's hebrew.py: a three-letter window and a flat
// chain of rules, each encoding an edge case a native speaker validated.
//
//   - https://en.wikipedia.org/wiki/Help:IPA/Hebrew
//   - https://hebrew-academy.org.il/2020/08/11/איך-הוגים-את-השווא-הנע
//   - https://hebrew-academy.org.il/2022/03/03/מלעיל-ומלרע-על-ההטעמה-בעברית

func phonemizeHebrew(letters []letter) []string {
	at := func(i int) *letter {
		if i < 0 || i >= len(letters) {
			return nil
		}
		return &letters[i]
	}
	var phonemes []string
	for i := 0; i < len(letters); {
		out, skipOffset := letterToPhonemes(letters[i], at(i-1), at(i+1))
		phonemes = append(phonemes, out...)
		i += skipOffset + 1
	}
	return phonemes
}

// handleYud reports a yod that is a mater, and so contributes no sound: bare, mid-word,
// not after an alef with tsere, and not before a vav with a meaning of its own.
func handleYud(cur letter, prev, next *letter) bool {
	return next != nil && prev != nil && cur.diac == "" && prev.char+prev.diac != "א\u05b5" &&
		(next.char != "ו" || next.diac == "" || strings.ContainsRune(next.diac, shva))
}

// handleVav resolves the vav: a consonant, either of two vowels, or silent.
func handleVav(cur letter, prev, next *letter) (phoneme string, skip bool, skipOffset int) {
	if prev != nil && strings.ContainsRune(prev.diac, shva) && strings.ContainsRune(cur.diac, holam) {
		return "vo", true, 0
	}

	if next != nil && next.char == "ו" {
		switch {
		case strings.ContainsRune(cur.diac+next.diac, holam):
			return "vo", true, 1
		case cur.diac == next.diac:
			return "vu", true, 1
		case strings.ContainsRune(cur.diac, hirik):
			return "vi", true, 0
		case strings.ContainsRune(cur.diac, shva) && next.diac == "":
			return "v", true, 0
		case strings.ContainsRune(cur.diac, kamatz) || strings.ContainsRune(cur.diac, patah):
			return "va", true, 0
		case strings.ContainsRune(cur.diac, tsere) || strings.ContainsRune(cur.diac, segol):
			return "ve", true, 0
		}
		return "", false, 0
	}

	// A single vav.
	switch {
	case strings.ContainsRune(cur.diac, patah) || strings.ContainsRune(cur.diac, kamatz):
		return "va", true, 0
	case strings.ContainsRune(cur.diac, tsere) || strings.ContainsRune(cur.diac, segol):
		return "ve", true, 0
	case strings.ContainsRune(cur.diac, holam):
		return "o", true, 0
	case strings.ContainsRune(cur.diac, kubuts) || strings.ContainsRune(cur.diac, dagesh):
		return "u", true, 0
	case strings.ContainsRune(cur.diac, shva) && prev == nil:
		return "ve", true, 0
	case strings.ContainsRune(cur.diac, hirik):
		return "vi", true, 0
	case next != nil && cur.diac == "":
		return "", true, 0
	}
	return "v", true, 0
}

// gnuvaPhonemes are the sounds a word-final patah takes under het, he and ayin.
var gnuvaPhonemes = map[string]string{"ח": "ax", "ה": "ah", "ע": "a"}

func letterToPhonemes(cur letter, prev, next *letter) ([]string, int) {
	var out []string
	skipConsonants, skipDiacritics, skipOffset := false, false, 0

	switch {
	case strings.ContainsRune(cur.allDiac, nikudHaserDiacritic):
		skipConsonants, skipDiacritics = true, true

	case cur.char == "א" && cur.diac == "" && prev != nil:
		skipConsonants = next != nil && next.char != "ו"

	case cur.char == "י" && handleYud(cur, prev, next):
		skipConsonants = true

	// A sin by its own dot, or a shin without nikud after one, which is itself a sin.
	case cur.char == "ש" && (strings.ContainsRune(cur.diac, sin) ||
		(cur.diac == "" && prev != nil && strings.ContainsRune(prev.diac, sin))):
		if next != nil && next.char == "ש" && next.diac == "" &&
			(strings.ContainsRune(cur.diac, patah) || strings.ContainsRune(cur.diac, kamatz)) {
			out = append(out, "sa") // יששכר
			skipConsonants, skipDiacritics = true, true
			skipOffset++
			break
		}
		out = append(out, "s")
		skipConsonants = true

	// A final patah is gnuva under these three letters.
	case next == nil && strings.ContainsRune(cur.diac, patah) && gnuvaPhonemes[cur.char] != "":
		out = append(out, gnuvaPhonemes[cur.char])
		skipConsonants, skipDiacritics = true, true
	}

	geresh, withDagesh := gereshPhonemes[cur.char], lettersPhonemes[cur.char+string(dagesh)]
	switch {
	case strings.ContainsRune(cur.diac, '\'') && geresh != "":
		out = append(out, geresh)
		skipConsonants = true
		skipDiacritics = skipDiacritics || cur.char == "ת"

	case strings.ContainsRune(cur.diac, dagesh) && withDagesh != "":
		out = append(out, withDagesh)
		skipConsonants = true

	case cur.char == "ו" && !strings.ContainsRune(cur.allDiac, nikudHaserDiacritic):
		vav, skip, off := handleVav(cur, prev, next)
		out = append(out, vav)
		skipConsonants, skipDiacritics, skipOffset = skip, skip, off
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

// sortStress moves the stress just before the vowel, where TTS expects it.
func sortStress(phonemes []string) []string {
	joined := strings.Join(phonemes, "")
	if !strings.Contains(joined, stressPhoneme) || !strings.ContainsAny(joined, "aeiou") {
		return phonemes
	}

	kept := slices.DeleteFunc(phonemes, func(p string) bool { return p == stressPhoneme })
	for i, p := range kept {
		if at := strings.IndexAny(p, "aeiou"); at >= 0 {
			kept[i] = p[:at] + stressPhoneme + p[at:]
			break
		}
	}
	return kept
}

// keepPhonemes drops non-phoneme strings: how letters, digits and quotes leave.
func keepPhonemes(phonemes []string) []string {
	return slices.DeleteFunc(phonemes, func(p string) bool {
		return p == "" || strings.ContainsFunc(p, func(r rune) bool { return !setPhonemes[r] })
	})
}
