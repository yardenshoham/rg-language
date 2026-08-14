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

// Title is the site's name.
const Title = "שפת הריש גימל"

// Layout wraps page content in the shared shell.
func Layout(title, currentPath string, children ...g.Node) g.Node {
	return c.HTML5(c.HTML5Props{
		Title:       title,
		Description: "כותבים משפט בעברית ומקבלים אותו בשפת הריש גימל, כתוב ומדובר.",
		Language:    "he",
		HTMLAttrs:   []g.Node{html.Dir("rtl")},
		Head: []g.Node{
			html.Link(html.Rel("stylesheet"), html.Href("/static/app.css")),
			html.Link(html.Rel("icon"), html.Href("/static/favicon.svg"), html.Type("image/svg+xml")),
			html.Script(html.Src("/static/htmx.min.js"), g.Attr("defer", "defer")),
		},
		Body: []g.Node{
			html.Header(
				html.A(html.Href("/"), html.H1(g.Text(Title))),
				navigation(currentPath),
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

// navigation is not called Nav: the html package already exports that name.
func navigation(currentPath string) g.Node {
	link := func(href, text string) g.Node {
		return html.A(html.Href(href), g.If(currentPath == href, html.Aria("current", "page")), g.Text(text))
	}
	return html.Nav(link("/", "תרגום"), link("/about", "מה זה"))
}
