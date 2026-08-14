package pages

import (
	g "maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// About explains the rule and is honest about what the voice cannot do.
func About() g.Node {
	return Layout("מה זה — "+Title, "/about",
		html.H2(g.Text("הכלל")),
		html.P(g.Text("אחרי כל תנועה נכנס רג ואחריו עוד עותק של אותה תנועה. זהו. שָׁלוֹם הופך ל־שָׁרְגָלוֹרְגוֹם, כי אחרי הָ נכנס רְגָ ואחרי לוֹ נכנס רְגוֹ.")),
		html.P(g.Text("מסתבר שלא צריך לחלק את המילה להברות בשביל זה. ההברה יכולה להיחתך לפני העיצור או אחריו, והתוצאה יוצאת אותו דבר — אז מספיק לדעת איפה התנועות.")),

		html.H2(g.Text("הדגש")),
		html.P(g.Text("התנועה המוטעמת מוכפלת, ואז צריך להחליט על מי מהשתיים נופל הדגש. כאן הוא נשאר על הראשונה, כלומר על המקורית, והרג נשאר חלש. זה מה שנשמע נכון לאוזן — בהאזנה עיוורת זו הייתה הבחירה ב־5 מתוך 6 מקרים.")),

		html.H2(g.Text("שלוש תצוגות")),
		html.Ul(
			html.Li(g.Text("בלי ניקוד — ככה אנשים באמת כותבים ריש גימל.")),
			html.Li(g.Text("עם ניקוד — כדי שיהיה ברור איך הוגים.")),
			html.Li(g.Text("באותיות לועזיות — למי שקל לו יותר ככה.")),
		),
		html.P(g.Text("החלק שנוסף מודגש בכל שלוש התצוגות.")),

		html.H2(g.Text("מה לא מושלם")),
		html.P(g.Text("הניקוד נקבע על ידי מודל, ולפעמים הוא טועה במילה בודדת שיש לה כמה קריאות — במשפט שלם הוא כמעט תמיד צודק. הקול סינתטי, ואותיות סוגרות כמו ד, ת, ק, ג ו־ב נבלעות לפעמים בסוף מילה. שתי הבעיות כמעט נעלמות במשפטים ארוכים, ולכן עדיף להקליד משפט ולא מילה בודדת.")),

		html.H2(g.Text("קרדיטים")),
		html.Ul(
			html.Li(
				html.A(g.Attr("href", "https://github.com/thewh1teagle/phonikud"), g.Text("phonikud")),
				g.Text(" — ניקוד אוטומטי והמרה לצלילים."),
			),
			html.Li(
				html.A(g.Attr("href", "https://github.com/rhasspy/piper"), g.Text("Piper")),
				g.Text(" — סינתזת הדיבור."),
			),
			html.Li(
				html.A(g.Attr("href", "https://fonts.google.com/noto/specimen/Noto+Sans+Hebrew"), g.Text("Noto Sans Hebrew")),
				g.Text(" — הגופן, כי ניקוד צריך גופן שיודע להציב אותו."),
			),
			html.Li(
				html.A(g.Attr("href", "https://www.gomponents.com"), g.Text("gomponents")),
				g.Text(" ו־"),
				html.A(g.Attr("href", "https://htmx.org"), g.Text("htmx")),
				g.Text(" — הדפים."),
			),
		),
		html.P(
			html.A(g.Attr("href", "https://github.com/yardenshoham/rg-language"),
				g.Text("github.com/yardenshoham/rg-language")),
		),
	)
}
