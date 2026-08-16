// Package phonikud converts vocalized Hebrew to IPA.
//
// A frozen fork of Python phonikud 0.4.1: the snapshot keeps the differential corpus
// valid. Left out: the digit-and-date expander (never tested end to end, so digits fall
// out of the stream) and the unused fallback/hyper-phoneme hooks.
package phonikud

import "strings"

// modernSchema rewrites the ASCII stand-ins into the symbols the voice was trained
// on. An unmapped symbol is silently dropped by pkg/voice.
var modernSchema = strings.NewReplacer(
	"x", "χ", // Het
	"r", "ʁ", // Resh
	"g", "ɡ", // Gimel
)

// Phonemize converts vocalized Hebrew, niqqud plus phonikud's extra marks, into IPA with stress.
func Phonemize(text string) string {
	return postClean(hePattern.ReplaceAllStringFunc(normalize(text), phonemizeWord))
}

func phonemizeWord(word string) string {
	// Upstream's mark_vocal_shva call here is a no-op; marks come from the diacritizer.
	if !strings.ContainsRune(word, hatamaDiacritic) {
		word = addMilraHatama(word)
	}
	letters := sortHatama(getLetters(word))
	phonemes := strings.Join(phonemizeHebrew(letters), "")
	return modernSchema.Replace(postNormalize(phonemes))
}

// postNormalize trims sounds Hebrew writes but modern speech drops word-finally.
func postNormalize(word string) string {
	word = strings.TrimSuffix(word, "ʔ") // no glottal stop at the end
	word = strings.TrimSuffix(word, "h") // no h at the end
	// Not redundant: two final h's leave "ˈh", and this takes the stray stress (תהה).
	word = strings.TrimSuffix(word, "ˈh")
	if rest, ok := strings.CutSuffix(word, "ij"); ok {
		word = rest + "i" // no j after an i
	}
	return word
}

// postClean drops everything that is not a phoneme and turns a hyphen into a break.
func postClean(phonemes string) string {
	var b strings.Builder
	for _, r := range phonemes {
		switch {
		case r == '-':
			b.WriteRune(' ')
		case setPhonemes[r] || strings.ContainsRune(".,!? ", r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
