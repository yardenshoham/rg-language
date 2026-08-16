package phonikud

// IPA for Hebrew consonants and vowels, ported verbatim from phonikud's lexicon.py.
// Marks are escapes throughout this package: as literals they attach to what precedes them.
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
	shva, hatafSegol, hatafPatah, hatafKamatz = '\u05b0', '\u05b1', '\u05b2', '\u05b3'
	hirik, tsere, segol, patah, kamatz        = '\u05b4', '\u05b5', '\u05b6', '\u05b7', '\u05b8'
	holam, holamHaser, kubuts, dagesh         = '\u05b9', '\u05ba', '\u05bb', '\u05bc'
	kamatzKatan, sin                          = '\u05c7', '\u05c2'
)

// hePatternText matches a run of Hebrew: letters, niqqud, phonikud's marks and quotes.
const hePatternText = "[\u05b0-ת\u05bd\u05ab|\u05af'\"]+"

var gereshPhonemes = map[string]string{
	"ג": "dʒ", "ז": "ʒ", "ת": "ta", "צ": "tʃ", "ץ": "tʃ",
}

// lettersPhonemes maps each consonant to its sound. Beged-kefet and shin/sin are
// keyed by letter plus mark.
var lettersPhonemes = map[string]string{
	"א": "ʔ", "ב": "v", "ג": "g", "ד": "d", "ה": "h", "ו": "v",
	"ז": "z", "ח": "x", "ט": "t", "י": "j", "ך": "x", "כ": "x",
	"ל": "l", "ם": "m", "מ": "m", "ן": "n", "נ": "n", "ס": "s",
	"ע": "ʔ", "פ": "f", "ף": "f", "ץ": "ts", "צ": "ts", "ק": "k",
	"ר": "r", "ש": "ʃ", "ת": "t",

	"ב\u05bc": "b", // Beged kefet: bet, kaf and pe with a dagesh
	"כ\u05bc": "k",
	"פ\u05bc": "p",

	"ש\u05c1": "ʃ", // Shin and sin, by their dot
	"ש\u05c2": "s",

	"'": "",
}

var nikudPhonemes = map[rune]string{
	hirik: "i", kubuts: "u", vocalShvaDiacritic: "e",
	hatafSegol: "e", tsere: "e", segol: "e",
	hatafPatah: "a", patah: "a", kamatz: "a",
	holam: "o", holamHaser: "o", hatafKamatz: "o", kamatzKatan: "o", // holamHaser: vav only
	hatamaDiacritic: stressPhoneme,
}

// setPhonemes is every output character, derived from the tables as phonikud does it.
var setPhonemes = func() map[rune]bool {
	set := map[rune]bool{'χ': true, 'ʁ': true, 'ɡ': true, 'w': true} // the modern schema, plus w
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
	return set
}()
