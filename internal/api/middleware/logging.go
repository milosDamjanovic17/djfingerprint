package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// responseRecorder wraps http.ResponseWriter to capture the status code
// and number of bytes written by the handler.
//
// The standard http.ResponseWriter does not expose these values after
// the handler completes — we need them to log the response details.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

// newResponseRecorder creates a responseRecorder with a default
// status code of 200. If the handler never calls WriteHeader,
// the implicit status is 200 OK.
func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader intercepts the status code before passing it through
// to the underlying ResponseWriter.
func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

// Write intercepts the response body bytes to count them,
// then passes through to the underlying ResponseWriter.
func (rr *responseRecorder) Write(b []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(b)
	rr.bytes += n
	return n, err
}

// Logging returns a middleware handler that logs every HTTP request
// and its outcome.
//
// On arrival it logs the method, path, and remote address.
// On completion it logs the status code, method, path, duration,
// and bytes written.
//
// Example output:
//
//	→ POST /api/v1/upload 192.168.1.1
//	← 201 POST /api/v1/upload 2.341s 142B
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		log.Printf("→ %s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

		rec := newResponseRecorder(w)
		next.ServeHTTP(rec, r)

		duration := time.Since(start)

		log.Printf(
			"← %d %s %s %s %s",
			rec.statusCode,
			r.Method,
			r.URL.Path,
			formatDuration(duration),
			formatBytes(rec.bytes),
		)
	})
}

// formatDuration formats a duration to a human-readable string.
// Shows milliseconds for short durations, seconds for longer ones.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.3fs", d.Seconds())
}

// formatBytes formats a byte count to a human-readable string.
func formatBytes(n int) string {
	return fmt.Sprintf("%dB", n)
}