// Package phonikud converts vocalized Hebrew to IPA.
//
// It is a frozen Go fork of the Python phonikud 0.4.1 finite-state transducer,
// deliberately not tracking upstream: a snapshot is what makes the differential
// corpus in testdata valid forever.
//
// Two features of the original are left out. The expander, which rewrites digits
// and dates into words, is skipped — numbers, dates and acronyms were never
// tested end to end in this project, so digits simply fall out of the phoneme
// stream instead. The fallback and hyper-phoneme (`[word](/ipa/)`) hooks are
// skipped too; nothing here uses them.
package phonikud

import "strings"

// modernSchema rewrites the ASCII stand-ins into the IPA symbols the voice was
// trained on.
var modernSchema = strings.NewReplacer(
	"x", "χ", // Het
	"r", "ʁ", // Resh
	"g", "ɡ", // Gimel
)

// Phonemize converts vocalized Hebrew — letters with niqqud, plus the stress and
// vocal-shva marks the diacritizer adds — into IPA with stress.
func Phonemize(text string) string {
	text = normalize(text)
	text = hePattern.ReplaceAllStringFunc(text, phonemizeWord)
	return postClean(text)
}

func phonemizeWord(word string) string {
	// Upstream also calls mark_vocal_shva(word) here and throws the result away,
	// so predicting vocal shva is a no-op. The marks come from the diacritizer
	// instead. Replicated by omission, on purpose.
	if !strings.ContainsRune(word, hatamaDiacritic) {
		word = addMilraHatama(word)
	}
	letters := sortHatama(getLetters(word))
	phonemes := strings.Join(phonemizeHebrew(letters), "")
	return modernSchema.Replace(postNormalize(phonemes))
}

// postNormalize trims the sounds that Hebrew writes but modern speech drops at
// the end of a word.
func postNormalize(phonemes string) string {
	words := strings.Split(phonemes, " ")
	for i, word := range words {
		word = strings.TrimSuffix(word, "ʔ") // no glottal stop at the end
		word = strings.TrimSuffix(word, "h") // no h at the end
		// Not redundant with the line above, however much it looks it: the first
		// trim removes one h, so a word ending in two of them is left as "ˈh",
		// and this takes the stray stress mark with it. תהה is such a word.
		word = strings.TrimSuffix(word, "ˈh")
		if rest, ok := strings.CutSuffix(word, "ij"); ok {
			word = rest + "i" // no j after an i
		}
		words[i] = word
	}
	return strings.Join(words, " ")
}

// postClean drops everything that is not a phoneme. Hebrew letters the rules had
// no sound for, digits and quotes all leave the stream here; a hyphen becomes a
// word break.
func postClean(phonemes string) string {
	var b strings.Builder
	for _, r := range phonemes {
		switch {
		case r == '-':
			b.WriteRune(' ')
		case setPhonemes[r] || isPunctuation(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
