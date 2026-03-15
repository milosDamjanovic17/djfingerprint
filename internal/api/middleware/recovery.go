package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recovery returns a middleware handler that recovers from panics
// in downstream handlers and returns HTTP 500 instead of crashing
// the request goroutine.
//
// When a panic occurs:
//  1. The panic value and full stack trace are logged
//  2. HTTP 500 Internal Server Error is returned to the client
//  3. The server continues processing other requests normally
//
// Without this middleware a panicking handler would crash its goroutine,
// leave the client connection hanging, and produce a confusing log output
// with no corresponding HTTP response.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic value and full stack trace
				log.Printf(
					"PANIC recovered: %v\n%s",
					err,
					debug.Stack(),
				)

				// Return a generic 500 response to the client
				// Do not expose internal panic details in the response body
				http.Error(
					w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError,
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}