package templates

import (
	"context"
	"io"

	"github.com/a-h/templ"
	"github.com/maxlesscode/ping-itor/internal/store"
)

// MonitorList renders the list fragment (used both in the full page and in
// HTMX partial responses for create/delete).
func MonitorList(monitors []store.Monitor) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		for _, m := range monitors {
			if err := MonitorRow(m).Render(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
}

// IndexPage renders the full index page inside the Layout.
func IndexPage(monitors []store.Monitor) templ.Component {
	inner := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := io.WriteString(w, `<div class="mb-8">
  <h2 class="text-lg font-semibold mb-4 text-neutral-200">Ajouter un moniteur</h2>
  <form
    hx-post="/monitors"
    hx-target="#monitor-list"
    hx-swap="innerHTML"
    hx-on::after-request="this.reset()"
    class="flex flex-col sm:flex-row gap-3"
  >
    <input
      type="text"
      name="name"
      placeholder="Nom du site"
      required
      class="flex-1 bg-neutral-800 border border-neutral-700 rounded-lg px-4 py-2 text-white placeholder-neutral-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
    />
    <input
      type="url"
      name="url"
      placeholder="https://exemple.com"
      required
      class="flex-1 bg-neutral-800 border border-neutral-700 rounded-lg px-4 py-2 text-white placeholder-neutral-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
    />
    <button
      type="submit"
      class="bg-blue-600 hover:bg-blue-500 text-white font-medium px-6 py-2 rounded-lg transition-colors"
    >Ajouter</button>
  </form>
</div>
<div id="monitor-list" hx-ext="sse" sse-connect="/events" class="space-y-0">
`)
		if err != nil {
			return err
		}
		for _, m := range monitors {
			if err = MonitorRow(m).Render(ctx, w); err != nil {
				return err
			}
		}
		_, err = io.WriteString(w, `</div>`)
		return err
	})

	return Layout("ping-itor", inner)
}
