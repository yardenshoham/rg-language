// Package pages renders the site.
//
// The whole site is Hebrew and right-to-left. Two things are load-bearing and
// easy to lose: the page must declare UTF-8 (the server sends the matching
// header), and it must use the bundled Noto Sans Hebrew at real weights.
// A faux-bold Hebrew font smears the niqqud, and the niqqud is the point of the
// pronunciation view.
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
		HTMLAttrs:   []g.Node{g.Attr("dir", "rtl")},
		Head: []g.Node{
			html.Link(g.Attr("rel", "stylesheet"), g.Attr("href", "/static/app.css")),
			html.Link(g.Attr("rel", "icon"), g.Attr("href", "/static/favicon.svg"),
				g.Attr("type", "image/svg+xml")),
			html.Script(g.Attr("src", "/static/htmx.min.js"), g.Attr("defer", "defer")),
		},
		Body: []g.Node{
			html.Header(
				html.A(g.Attr("href", "/"), html.H1(g.Text(Title))),
				navigation(currentPath),
			),
			html.Main(g.Group(children)),
			html.Footer(
				html.P(
					g.Text("צעצוע. הדיבור נוצר במחשב, אז לפעמים אות אחרונה נבלעת. "),
					html.A(g.Attr("href", "/about"), g.Text("עוד על זה")),
					g.Text("."),
				),
			),
		},
	})
}

// navigation is not called Nav: the html package already exports that name.
func navigation(currentPath string) g.Node {
	link := func(href, text string) g.Node {
		return html.A(g.Attr("href", href),
			g.If(currentPath == href, g.Attr("aria-current", "page")),
			g.Text(text),
		)
	}
	return html.Nav(link("/", "תרגום"), link("/about", "מה זה"))
}
