package phonikud

// ASCII IPA transcription of Hebrew consonants and vowels, ported verbatim from
// phonikud's lexicon.py.
//
// Combining marks are written as \u escapes throughout this package: as literal
// characters they attach to whatever precedes them in the source and become
// impossible to edit reliably.
//
// Reference:
// https://en.wikipedia.org/wiki/Unicode_and_HTML_for_the_Hebrew_alphabet#Compact_table

// phonikud's extra diacritics. The diacritizer emits the first three; masora and
// the geresh come from the text.
const (
	vocalShvaDiacritic  = '\u05bd' // meteg
	hatamaDiacritic     = '\u05ab' // ole
	prefixDiacritic     = '|'      // vertical bar
	nikudHaserDiacritic = '\u05af' // masora, not in use

	stressPhoneme = "ˈ" // ˈ, visually looks like a single quote
)

// Standard niqqud, by name.
const (
	shva        = '\u05b0'
	hatafSegol  = '\u05b1'
	hatafPatah  = '\u05b2'
	hatafKamatz = '\u05b3'
	hirik       = '\u05b4'
	tsere       = '\u05b5'
	segol       = '\u05b6'
	patah       = '\u05b7'
	kamatz      = '\u05b8'
	holam       = '\u05b9'
	holamHaser  = '\u05ba'
	kubuts      = '\u05bb'
	dagesh      = '\u05bc'
	kamatzKatan = '\u05c7'
	sin         = '\u05c2'
)

// hePatternText matches a run of Hebrew: niqqud and letters, ole, meteg, masora,
// the prefix bar, the en geresh and a double quote.
const hePatternText = "[\u05b0-ת\u05bd\u05ab|\u05af'\"]+"

// gereshPhonemes are the sounds a geresh adds to a letter.
var gereshPhonemes = map[string]string{
	"ג": "dʒ", "ז": "ʒ", "ת": "ta", "צ": "tʃ", "ץ": "tʃ",
}

// lettersPhonemes maps each consonant to its sound. Beged-kefet and shin/sin are
// keyed by letter plus mark.
var lettersPhonemes = map[string]string{
	"א": "ʔ",  // Alef
	"ב": "v",  // Bet
	"ג": "g",  // Gimel
	"ד": "d",  // Dalet
	"ה": "h",  // He
	"ו": "v",  // Vav
	"ז": "z",  // Zayin
	"ח": "x",  // Het
	"ט": "t",  // Tet
	"י": "j",  // Yod
	"ך": "x",  // Haf sofit
	"כ": "x",  // Haf
	"ל": "l",  // Lamed
	"ם": "m",  // Mem sofit
	"מ": "m",  // Mem
	"ן": "n",  // Nun sofit
	"נ": "n",  // Nun
	"ס": "s",  // Samekh
	"ע": "ʔ",  // Ayin, only voweled
	"פ": "f",  // Fey
	"ף": "f",  // Fey sofit
	"ץ": "ts", // Tsadik sofit
	"צ": "ts", // Tsadik
	"ק": "k",  // Kuf
	"ר": "r",  // Resh
	"ש": "ʃ",  // Shin
	"ת": "t",  // Taf

	"ב\u05bc": "b", // Beged kefet: bet, kaf and pe with a dagesh
	"כ\u05bc": "k",
	"פ\u05bc": "p",

	"ש\u05c1": "ʃ", // Shin and sin, by their dot
	"ש\u05c2": "s",

	"'": "",
}

// nikudPhonemes maps each vowel mark to its sound.
var nikudPhonemes = map[rune]string{
	hirik:              "i",
	hatafSegol:         "e",
	tsere:              "e",
	segol:              "e",
	hatafPatah:         "a",
	patah:              "a",
	kamatzKatan:        "o",
	holam:              "o",
	holamHaser:         "o", // for vav
	kubuts:             "u",
	hatafKamatz:        "o",
	kamatz:             "a",
	hatamaDiacritic:    stressPhoneme,
	vocalShvaDiacritic: "e",
}

// isEnhancedDiacritic reports a mark meaningful to the transducer but not part of
// the letter's vowel. letter.diac excludes these.
func isEnhancedDiacritic(r rune) bool {
	return r == hatamaDiacritic || r == prefixDiacritic || r == vocalShvaDiacritic
}

// setPhonemes is every character that may appear in the output, derived from the
// tables above exactly as phonikud derives it. Anything else is dropped.
var setPhonemes = func() map[rune]bool {
	set := map[rune]bool{}
	add := func(s string) {
		if r := []rune(s); len(r) == 1 {
			set[r[0]] = true
		}
	}
	for _, v := range nikudPhonemes {
		add(v)
	}
	for _, table := range []map[string]string{lettersPhonemes, gereshPhonemes} {
		for _, v := range table {
			add(v)
		}
	}
	for _, r := range "χʁɡw" { // the modern schema, plus the special phoneme w
		set[r] = true
	}
	return set
}()

func isPunctuation(r rune) bool {
	return r == '.' || r == ',' || r == '!' || r == '?' || r == ' '
}
