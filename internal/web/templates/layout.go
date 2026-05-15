// Package templates provides templ components for the ping-itor web UI.
package templates

import (
	"context"
	"html"
	"io"

	"github.com/a-h/templ"
)

// Layout wraps the given inner component in a full HTML page skeleton
// with Tailwind CSS, HTMX v2, and the HTMX SSE extension.
func Layout(title string, inner templ.Component) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w, `<!doctype html>
<html lang="fr" class="h-full bg-neutral-950">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>`+html.EscapeString(title)+`</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <script src="https://unpkg.com/htmx.org@2.0.4" integrity="sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+" crossorigin="anonymous"></script>
  <script src="https://unpkg.com/htmx-ext-sse@2.2.2/sse.js"></script>
</head>
<body class="h-full text-neutral-100 font-sans antialiased">
  <div class="min-h-full">
    <header class="bg-neutral-900 border-b border-neutral-800 py-4 px-6">
      <h1 class="text-xl font-bold tracking-tight text-white">ping-itor</h1>
    </header>
    <main class="max-w-5xl mx-auto px-4 py-8">
`)
		if err != nil {
			return err
		}
		if err = inner.Render(ctx, w); err != nil {
			return err
		}
		_, err = io.WriteString(w, `
    </main>
  </div>
</body>
</html>`)
		return err
	})
}
