package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/maxlesscode/ping-itor/internal/store"
	"github.com/maxlesscode/ping-itor/internal/web/templates"
)

// HandlerInput groups the dependencies required by the HTTP handler.
type HandlerInput struct {
	Store       *store.Store
	Broadcaster *Broadcaster
}

// NewHandler wires the chi router with all application routes and returns it
// as an http.Handler.
func NewHandler(input HandlerInput) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	h := &handler{
		store:       input.Store,
		broadcaster: input.Broadcaster,
	}

	r.Get("/", h.index)
	r.Get("/home", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	})
	r.Post("/monitors", h.createMonitor)
	r.Delete("/monitors/{id}", h.deleteMonitor)
	r.Get("/events", h.events)

	return r
}

type handler struct {
	store       *store.Store
	broadcaster *Broadcaster
}

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.store.List(r.Context())
	if err != nil {
		slog.Error("handler: list monitors", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err = templates.IndexPage(monitors).Render(r.Context(), w); err != nil {
		slog.Error("handler: render index", "err", err)
	}
}

func (h *handler) createMonitor(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	url := r.FormValue("url")

	if name == "" || url == "" {
		http.Error(w, "name and url are required", http.StatusBadRequest)
		return
	}

	if _, err := h.store.Create(r.Context(), name, url); err != nil {
		slog.Error("handler: create monitor", "name", name, "url", url, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.renderMonitorList(w, r)
}

func (h *handler) deleteMonitor(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err = h.store.Delete(r.Context(), id); err != nil {
		slog.Error("handler: delete monitor", "id", id, "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.renderMonitorList(w, r)
}

func (h *handler) renderMonitorList(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.store.List(r.Context())
	if err != nil {
		slog.Error("handler: list monitors after mutation", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err = templates.MonitorList(monitors).Render(r.Context(), w); err != nil {
		slog.Error("handler: render monitor list", "err", err)
	}
}

func (h *handler) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	ch, unsub := h.broadcaster.Subscribe()
	defer unsub()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case result, ok := <-ch:
			if !ok {
				return
			}
			m, err := h.store.Get(ctx, result.MonitorID)
			if err != nil {
				slog.Error("events: get monitor", "id", result.MonitorID, "err", err)
				continue
			}

			html, err := renderToString(ctx, templates.MonitorRow(m))
			if err != nil {
				slog.Error("events: render monitor row", "id", m.ID, "err", err)
				continue
			}

			fmt.Fprintf(w, "event: monitor-%d\ndata: %s\n\n", m.ID, strings.ReplaceAll(html, "\n", ""))
			flusher.Flush()
		}
	}
}

// renderToString renders a templ component to a string suitable for an SSE
// data field.
func renderToString(ctx context.Context, c templ.Component) (string, error) {
	var buf bytes.Buffer
	if err := c.Render(ctx, io.Writer(&buf)); err != nil {
		return "", err
	}
	return buf.String(), nil
}
