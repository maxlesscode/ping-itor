package templates

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/a-h/templ"
	"github.com/maxlesscode/ping-itor/internal/store"
)

// MonitorRow renders a single monitor as an SSE-swappable table row div.
func MonitorRow(m store.Monitor) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		statusDot := `<span class="text-3xl text-neutral-500" title="Jamais vérifié">&#11044;</span>`
		if m.IsUp != nil {
			if *m.IsUp {
				statusDot = `<span class="text-3xl text-green-500" title="En ligne">&#11044;</span>`
			} else {
				statusDot = `<span class="text-3xl text-red-500" title="Hors ligne">&#11044;</span>`
			}
		}

		lastChecked := "Jamais"
		if m.LastChecked != nil {
			lastChecked = htmlEscape(m.LastChecked.Local().Format(time.DateTime))
		}

		latency := "—"
		if m.LatencyMs != nil {
			latency = fmt.Sprintf("%d ms", *m.LatencyMs)
		}

		httpCode := "—"
		if m.HTTPCode != nil {
			httpCode = fmt.Sprintf("%d", *m.HTTPCode)
		}

		html := fmt.Sprintf(`<div id="monitor-row-%d" sse-swap="monitor-%d" hx-swap="outerHTML" class="bg-neutral-900 rounded-lg border border-neutral-800 p-4 flex items-center gap-4 mb-3">
  <div class="flex-shrink-0">%s</div>
  <div class="flex-1 min-w-0">
    <div class="flex items-center gap-2">
      <span class="font-semibold text-white truncate">%s</span>
      <a href="%s" target="_blank" rel="noopener noreferrer"
         class="text-neutral-400 hover:text-blue-400 text-sm truncate max-w-xs transition-colors">%s</a>
    </div>
    <div class="flex gap-4 mt-1 text-sm text-neutral-400">
      <span>Dernier contrôle: <span class="text-neutral-300">%s</span></span>
      <span>Latence: <span class="text-neutral-300">%s</span></span>
      <span>HTTP: <span class="text-neutral-300">%s</span></span>
    </div>
  </div>
  <div class="flex-shrink-0">
    <button
      hx-delete="/monitors/%d"
      hx-target="#monitor-list"
      hx-swap="innerHTML"
      hx-confirm="Supprimer ce moniteur ?"
      class="text-neutral-500 hover:text-red-400 transition-colors px-2 py-1 rounded text-sm"
    >Supprimer</button>
  </div>
</div>`,
			m.ID, m.ID,
			statusDot,
			htmlEscape(m.Name),
			htmlEscape(m.URL), htmlEscape(m.URL),
			lastChecked,
			latency,
			httpCode,
			m.ID,
		)

		_, err := io.WriteString(w, html)
		return err
	})
}
