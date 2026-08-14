package pages

import (
	"net/url"

	g "maragu.dev/gomponents"
	htmx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"

	"github.com/yardenshoham/rg-language/pkg/heb"
	"github.com/yardenshoham/rg-language/pkg/pipeline"
	"github.com/yardenshoham/rg-language/pkg/rg"
)

// Examples are the project's own reference sentences. They are whole sentences
// on purpose: the voice is at its best in connected speech and at its weakest on
// one- and two-syllable words, so the site nudges people towards sentences.
var Examples = []string{
	"מה נשמע",
	"היום יום שלישי",
	"אני ממש אוהב פיצה",
	"בוקר טוב לכולם",
	"אמא קנתה לי גלידה בשוק",
}

// Home renders the whole page. result is the zero value before anything is typed.
func Home(text string, result pipeline.Result) g.Node {
	return Layout(Title, "/",
		html.P(g.Attr("class", "lead"),
			g.Text("כותבים משפט בעברית, ומקבלים אותו בשפת הריש גימל — אחרי כל תנועה נכנס "),
			html.Strong(g.Text("רג")),
			g.Text(" ועוד עותק של אותה תנועה. אפשר גם לשמוע."),
		),

		html.Form(g.Attr("method", "get"), g.Attr("action", "/"),
			// Typing updates the result live; pressing the button does a plain
			// navigation, so the URL becomes a link worth sharing.
			htmx.Get("/transform"),
			htmx.Trigger("input changed delay:500ms"),
			htmx.Target("#result"),
			htmx.Indicator("#result"),
			html.Textarea(
				g.Attr("name", "text"), g.Attr("id", "text"), g.Attr("dir", "rtl"),
				g.Attr("lang", "he"), g.Attr("rows", "3"), g.Attr("autofocus", "autofocus"),
				g.Attr("maxlength", "500"), g.Attr("aria-label", "משפט בעברית"),
				g.Attr("placeholder", "אני ממש אוהב פיצה"),
				g.Text(text),
			),
			html.Button(g.Attr("type", "submit"), g.Text("תרגמו")),
		),

		html.P(g.Attr("class", "examples"),
			g.Text("לדוגמה: "),
			g.Group(g.Map(Examples, func(example string) g.Node {
				return html.A(g.Attr("href", "/?text="+url.QueryEscape(example)), g.Text(example))
			})),
		),

		html.Div(g.Attr("id", "result"), Result(result)),
	)
}

// Result renders the three views plus the player. It is also the htmx fragment,
// so the page and the live update cannot drift apart.
func Result(result pipeline.Result) g.Node {
	if result.Input == "" {
		return nil
	}

	return g.Group([]g.Node{
		view("ככה כותבים את זה", "plain", "rtl", hebrewSegments(result.Unvocalized())),
		view("ככה הוגים את זה", "vocalized", "rtl", hebrewSegments(result.Hebrew)),
		view("ובאותיות לועזיות", "latin", "ltr", latinSyllables(result.Syllables)),
		html.Div(g.Attr("class", "player"),
			html.Audio(
				g.Attr("controls", "controls"), g.Attr("preload", "none"),
				g.Attr("src", "/audio/"+result.AudioHash+".wav"),
				g.Text("הדפדפן שלכם לא יודע לנגן את הקובץ."),
			),
		),
	})
}

func view(label, class, dir string, body g.Node) g.Node {
	return html.Section(g.Attr("class", "view "+class),
		html.H2(g.Text(label)),
		html.P(g.Attr("dir", dir), body),
	)
}

// hebrewSegments marks up the inserted רג so it can be tinted. The runs are
// already merged, so this is one span per alternation, not one per letter.
func hebrewSegments(segments []rg.Segment) g.Node {
	return g.Group(g.Map(segments, func(s rg.Segment) g.Node {
		if !s.Inserted {
			return g.Text(s.Text)
		}
		return html.Span(g.Attr("class", "inserted"), g.Text(s.Text))
	}))
}

// latinSyllables hyphenates the words and tints the inserted syllables, which
// under a maximal-onset split are exactly the ones starting with רג.
func latinSyllables(words [][]heb.Syllable) g.Node {
	nodes := make([]g.Node, 0, len(words))
	for i, word := range words {
		if i > 0 {
			nodes = append(nodes, g.Text(" "))
		}
		for j, syllable := range word {
			if j > 0 {
				nodes = append(nodes, g.Text("-"))
			}
			switch {
			case syllable.Inserted:
				nodes = append(nodes, html.Span(g.Attr("class", "inserted"), g.Text(syllable.Text)))
			case syllable.Stressed:
				nodes = append(nodes, html.Span(g.Attr("class", "stressed"), g.Text(syllable.Text)))
			default:
				nodes = append(nodes, g.Text(syllable.Text))
			}
		}
	}
	return g.Group(nodes)
}
