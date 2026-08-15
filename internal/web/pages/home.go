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

// Examples are whole sentences on purpose: the voice is worst on one- and two-syllable words.
var Examples = []string{
	"מה נשמע",
	"היום יום שלישי",
	"אני ממש אוהב פיצה",
	"בוקר טוב לכולם",
	"אמא קנתה לי גלידה בשוק",
}

// Home renders the whole page. result is the zero value before anything is typed.
func Home(analytics Analytics, text string, result pipeline.Result) g.Node {
	return Layout(analytics, Title, "/",
		html.P(html.Class("lead"),
			g.Text("כותבים משפט בעברית, ומקבלים אותו בשפת הריש גימל — אחרי כל תנועה נכנס "),
			html.Strong(g.Text("רג")),
			g.Text(" ועוד עותק של אותה תנועה. אפשר גם לשמוע."),
		),

		html.Form(html.Method("get"), html.Action("/"),
			// Typing updates live; the button navigates, making a shareable URL.
			htmx.Get("/transform"),
			htmx.Trigger("input changed delay:500ms"),
			htmx.Target("#result"),
			htmx.Indicator("#result"),
			html.Textarea(
				html.Name("text"), html.ID("text"), html.Dir("rtl"), html.Lang("he"),
				html.Rows("3"), g.Attr("autofocus", "autofocus"), html.MaxLength("500"),
				html.Aria("label", "משפט בעברית"), html.Placeholder("אני ממש אוהב פיצה"),
				g.Text(text),
			),
			html.Button(html.Type("submit"), g.Text("תרגמו")),
		),

		html.P(html.Class("examples"), g.Text("לדוגמה: "),
			g.Map(Examples, func(example string) g.Node {
				return html.A(html.Href("/?text="+url.QueryEscape(example)), g.Text(example))
			}),
		),

		html.Div(html.ID("result"), Result(result)),
	)
}

// Result is also the htmx fragment, so the page and the live update cannot drift apart.
func Result(result pipeline.Result) g.Node {
	if result.Input == "" {
		// An empty fragment is how htmx clears the result when the box is emptied.
		return g.Group{}
	}

	return g.Group{
		view("ככה כותבים את זה", "plain", "rtl", hebrewSegments(result.Unvocalized())),
		view("ככה הוגים את זה", "vocalized", "rtl", hebrewSegments(result.Hebrew)),
		view("ובאותיות לועזיות", "latin", "ltr", latinSyllables(result.Syllables)),
		html.Div(html.Class("player"), html.Audio(
			g.Attr("controls", "controls"), html.Preload("none"), html.Src("/audio/"+result.AudioHash+".wav"),
			g.Text("הדפדפן שלכם לא יודע לנגן את הקובץ."))),
	}
}

func view(label, class, dir string, body g.Node) g.Node {
	return html.Section(html.Class("view "+class),
		html.H2(g.Text(label)),
		html.P(html.Dir(dir), body),
	)
}

// mark tints an inserted or a stressed piece; inserted wins when a piece is both.
func mark(text string, inserted, stressed bool) g.Node {
	switch {
	case inserted:
		return html.Span(html.Class("inserted"), g.Text(text))
	case stressed:
		return html.Span(html.Class("stressed"), g.Text(text))
	}
	return g.Text(text)
}

// hebrewSegments tints the inserted רג. Runs arrive merged, so one span per alternation.
func hebrewSegments(segments []rg.Segment) g.Node {
	return g.Map(segments, func(s rg.Segment) g.Node { return mark(s.Text, s.Inserted, false) })
}

func latinSyllables(words [][]heb.Syllable) g.Node {
	var nodes g.Group
	for i, word := range words {
		nodes = append(nodes, g.If(i > 0, g.Text(" ")))
		for j, syllable := range word {
			nodes = append(nodes, g.If(j > 0, g.Text("-")), mark(syllable.Text, syllable.Inserted, syllable.Stressed))
		}
	}
	return nodes
}
