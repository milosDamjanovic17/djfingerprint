package middleware

import (
	"net/http"
	"strings"
)

// corsConfig holds the CORS policy for the API.
// These values are appropriate for local development with
// a Vite dev server on port 5173.
//
// For production these values should come from configuration.
var corsConfig = struct {
	allowedOrigins []string
	allowedMethods string
	allowedHeaders string
	maxAge         string
}{
	allowedOrigins: []string{
		"http://localhost:5173", // Vite dev server
		"http://localhost:3000", // Alternative dev port
		"http://localhost:8080", // Same-origin requests
	},
	allowedMethods: "GET, POST, DELETE, OPTIONS",
	allowedHeaders: "Content-Type, Authorization, Accept",
	maxAge:         "86400", // 24 hours — browsers cache preflight results
}

// CORS returns a middleware handler that sets Cross-Origin Resource
// Sharing headers on every response.
//
// This allows the Vue frontend running on a different port to make
// API calls without the browser blocking them.
//
// OPTIONS preflight requests are handled and terminated here —
// they receive HTTP 200 with the CORS headers and do not reach
// the downstream handler.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Set CORS headers if the request origin is allowed
		// or if there is no origin (same-origin / non-browser request)
		if origin == "" || isAllowedOrigin(origin) {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		} else {
			// Origin not allowed — still process the request
			// but without CORS headers the browser will block it
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Methods", corsConfig.allowedMethods)
		w.Header().Set("Access-Control-Allow-Headers", corsConfig.allowedHeaders)
		w.Header().Set("Access-Control-Max-Age", corsConfig.maxAge)

		// Handle OPTIONS preflight requests
		// The browser sends OPTIONS before any cross-origin POST or DELETE
		// to check whether the actual request is permitted.
		// We respond 200 and stop — do not pass OPTIONS to the handler.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks whether the request origin is in the
// allowed origins list.
func isAllowedOrigin(origin string) bool {
	for _, allowed := range corsConfig.allowedOrigins {
		if strings.EqualFold(origin, allowed) {
			return true
		}
	}
	return false
}