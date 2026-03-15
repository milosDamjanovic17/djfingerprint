package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// -----------------------------------------------------------------------
// responseRecorder
// -----------------------------------------------------------------------

func TestResponseRecorder_DefaultStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	rr := newResponseRecorder(w)

	// Default status must be 200 when WriteHeader is never called
	if rr.statusCode != http.StatusOK {
		t.Errorf("expected default status 200, got %d", rr.statusCode)
	}
}

func TestResponseRecorder_CapturesStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	rr := newResponseRecorder(w)

	rr.WriteHeader(http.StatusCreated)

	if rr.statusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.statusCode)
	}
}

func TestResponseRecorder_CapturesBytes(t *testing.T) {
	w := httptest.NewRecorder()
	rr := newResponseRecorder(w)

	body := []byte("hello world")
	rr.Write(body)

	if rr.bytes != len(body) {
		t.Errorf("expected %d bytes, got %d", len(body), rr.bytes)
	}
}

func TestResponseRecorder_PassesThroughToUnderlying(t *testing.T) {
	w := httptest.NewRecorder()
	rr := newResponseRecorder(w)

	rr.WriteHeader(http.StatusNotFound)
	rr.Write([]byte("not found"))

	if w.Code != http.StatusNotFound {
		t.Errorf("expected underlying recorder code 404, got %d", w.Code)
	}
	if w.Body.String() != "not found" {
		t.Errorf("expected body 'not found', got %q", w.Body.String())
	}
}

// -----------------------------------------------------------------------
// Logging Middleware
// -----------------------------------------------------------------------

func TestLogging_PassesThroughResponse(t *testing.T) {
	// The logging middleware must not alter the handler's response
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}
	if rec.Body.String() != "created" {
		t.Errorf("expected body 'created', got %q", rec.Body.String())
	}
}

func TestLogging_200StatusPassesThrough(t *testing.T) {
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestLogging_404StatusPassesThrough(t *testing.T) {
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestLogging_500StatusPassesThrough(t *testing.T) {
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestLogging_WorksForGetMethod(t *testing.T) {
	called := false
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called for GET request")
	}
}

func TestLogging_WorksForDeleteMethod(t *testing.T) {
	called := false
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1", nil)
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected handler to be called for DELETE request")
	}
}

func TestLogging_BodyContentReachesClient(t *testing.T) {
	const body = `{"id":1,"source":"manual"}`

	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Body.String() != body {
		t.Errorf("expected body %q, got %q", body, rec.Body.String())
	}
}

// -----------------------------------------------------------------------
// formatDuration
// -----------------------------------------------------------------------

func TestFormatDuration_Milliseconds(t *testing.T) {
	result := formatDuration(500 * 1000000) // 500ms in nanoseconds
	if result != "500ms" {
		t.Errorf("expected '500ms', got %q", result)
	}
}

func TestFormatDuration_Seconds(t *testing.T) {
	result := formatDuration(2 * 1000000000) // 2 seconds in nanoseconds
	if result != "2.000s" {
		t.Errorf("expected '2.000s', got %q", result)
	}
}

// -----------------------------------------------------------------------
// formatBytes
// -----------------------------------------------------------------------

func TestFormatBytes_Zero(t *testing.T) {
	result := formatBytes(0)
	if result != "0B" {
		t.Errorf("expected '0B', got %q", result)
	}
}

func TestFormatBytes_NonZero(t *testing.T) {
	result := formatBytes(142)
	if result != "142B" {
		t.Errorf("expected '142B', got %q", result)
	}
}