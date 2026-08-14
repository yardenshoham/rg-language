// Package phonikud converts vocalized Hebrew to IPA.
//
// A frozen Go fork of the Python phonikud 0.4.1 transducer, deliberately not
// tracking upstream: the snapshot is what keeps the differential corpus valid
// forever. Two upstream features are left out — the digit-and-date expander
// (numbers, dates and acronyms were never tested end to end here, so digits just
// fall out of the stream) and the fallback and hyper-phoneme (`[word](/ipa/)`)
// hooks, which nothing uses.
package phonikud

import "strings"

// modernSchema rewrites the ASCII stand-ins into the symbols the voice was trained
// on. An unmapped symbol is silently dropped by pkg/voice.
var modernSchema = strings.NewReplacer(
	"x", "χ", // Het
	"r", "ʁ", // Resh
	"g", "ɡ", // Gimel
)

// Phonemize converts vocalized Hebrew — niqqud plus the diacritizer's stress and
// vocal-shva marks — into IPA with stress.
func Phonemize(text string) string {
	text = normalize(text)
	text = hePattern.ReplaceAllStringFunc(text, phonemizeWord)
	return postClean(text)
}

func phonemizeWord(word string) string {
	// Upstream calls mark_vocal_shva here and throws the result away, so it is a
	// no-op; the marks come from the diacritizer. Replicated by omission.
	if !strings.ContainsRune(word, hatamaDiacritic) {
		word = addMilraHatama(word)
	}
	letters := sortHatama(getLetters(word))
	phonemes := strings.Join(phonemizeHebrew(letters), "")
	return modernSchema.Replace(postNormalize(phonemes))
}

// postNormalize trims sounds Hebrew writes but modern speech drops word-finally.
func postNormalize(phonemes string) string {
	words := strings.Split(phonemes, " ")
	for i, word := range words {
		word = strings.TrimSuffix(word, "ʔ") // no glottal stop at the end
		word = strings.TrimSuffix(word, "h") // no h at the end
		// Not redundant with the line above: the first trim leaves a word ending in
		// two h's as "ˈh", and this takes the stray stress with it. תהה is one.
		word = strings.TrimSuffix(word, "ˈh")
		if rest, ok := strings.CutSuffix(word, "ij"); ok {
			word = rest + "i" // no j after an i
		}
		words[i] = word
	}
	return strings.Join(words, " ")
}

// postClean drops everything that is not a phoneme — unsounded Hebrew letters,
// digits, quotes — and turns a hyphen into a word break.
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
