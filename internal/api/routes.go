package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/milosDamjanovic17/djfingerprint/internal/api/handlers"
	"github.com/milosDamjanovic17/djfingerprint/internal/api/middleware"
)

// NewRouter builds and returns the application HTTP router.
//
// All routes are mounted under /api/v1. Middleware is applied at the
// router level so every request passes through the full stack regardless
// of which route is matched.
//
// The returned type is http.Handler rather than *chi.Mux so that callers
// (main.go, tests) depend only on the standard library interface and are
// not coupled to chi's concrete type.
func NewRouter(h *handlers.Handler) http.Handler {
	r := chi.NewRouter()

	// ── Middleware stack ─────────────────────────────────────────────
	//
	// Order is deliberate:
	//   1. Recovery  — catches panics from any middleware or handler below
	//   2. Logging   — logs every request and its outcome
	//   3. CORS      — sets CORS headers before any handler writes a response
	r.Use(middleware.Recovery)
	r.Use(middleware.Logging)
	r.Use(middleware.CORS)

	// ── Routes ──────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		// Upload a full song for fingerprint registration.
		// Returns 201 Created with the song record on success.
		r.Post("/upload", h.Upload)

		// Identify a short audio snippet against the library.
		// Always returns 200 — found:true or found:false.
		r.Post("/identify", h.Identify)

		// Song library management.
		r.Get("/songs", h.ListSongs)

		r.Route("/songs/{id}", func(r chi.Router) {
			r.Get("/", h.GetSong)
			r.Delete("/", h.DeleteSong)
		})
	})

	return r
}