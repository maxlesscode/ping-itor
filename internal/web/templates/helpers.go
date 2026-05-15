package templates

import "html"

// htmlEscape is a package-local alias so component files stay readable.
func htmlEscape(s string) string {
	return html.EscapeString(s)
}
