// Package pages renders the site: Hebrew, right-to-left. Two things are
// load-bearing and easy to lose — the page must declare UTF-8 (the server sends
// the matching header), and it must use the bundled Noto Sans Hebrew at real
// weights, because faux bold smears the niqqud that the whole view is for.
package pages

import (
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	"maragu.dev/gomponents/html"
)

const Title = "שפת הריש גימל"

// Layout threads analytics in rather than keeping it in a package variable, so two
// servers in one process — which is what the tests are — cannot see each other's.
func Layout(analytics Analytics, title, currentPath string, children ...g.Node) g.Node {
	link := func(href, text string) g.Node {
		return html.A(html.Href(href), g.If(currentPath == href, html.Aria("current", "page")), g.Text(text))
	}
	return c.HTML5(c.HTML5Props{
		Title:       title,
		Description: "כותבים משפט בעברית ומקבלים אותו בשפת הריש גימל, כתוב ומדובר.",
		Language:    "he",
		HTMLAttrs:   []g.Node{html.Dir("rtl")},
		Head: []g.Node{
			html.Link(html.Rel("stylesheet"), html.Href("/static/app.css")),
			html.Link(html.Rel("icon"), html.Href("/static/favicon.svg"), html.Type("image/svg+xml")),
			html.Script(html.Src("/static/htmx.min.js"), g.Attr("defer", "defer")),
			posthogScript(analytics),
		},
		Body: []g.Node{
			html.Header(
				html.A(html.Href("/"), html.H1(g.Text(Title))),
				html.Nav(link("/", "תרגום"), link("/about", "מה זה")),
			),
			html.Main(children...),
			html.Footer(
				html.P(
					g.Text("צעצוע. הדיבור נוצר במחשב, אז לפעמים אות אחרונה נבלעת. "),
					html.A(html.Href("/about"), g.Text("עוד על זה")),
					g.Text("."),
				),
			),
		},
	})
}
