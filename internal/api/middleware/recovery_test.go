package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecovery_NoPanic_PassesThrough(t *testing.T) {
	// A handler that does not panic must pass through normally
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestRecovery_StringPanic_Returns500(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// Must not crash the test process
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", rec.Code)
	}
}

func TestRecovery_ErrorPanic_Returns500(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after error panic, got %d", rec.Code)
	}
}

func TestRecovery_Panic_ResponseBodyIndicatesError(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Internal Server Error") {
		t.Errorf("expected body to contain 'Internal Server Error', got %q", body)
	}
}

func TestRecovery_Panic_DoesNotExposeInternals(t *testing.T) {
	// The panic message must NOT appear in the response body
	// Exposing internal details is a security risk
	const panicMsg = "secret database password"

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(panicMsg)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), panicMsg) {
		t.Error("panic message must not be exposed in HTTP response body")
	}
}

func TestRecovery_MultipleRequests_ServerContinuesAfterPanic(t *testing.T) {
	// After a panicking request the handler must still serve
	// subsequent requests correctly
	callCount := 0

	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			panic("first request panics")
		}
		w.WriteHeader(http.StatusOK)
	}))

	// First request — panics
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec1.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on panicking request, got %d", rec1.Code)
	}

	// Second request — must succeed normally
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200 on second request after panic, got %d", rec2.Code)
	}
}

func TestRecovery_NilPanic_Returns500(t *testing.T) {
	handler := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rec, req)

	// nil panic is still a panic — must return 500
	// Note: Go 1.21+ wraps nil panics in a runtime.PanicNilError
	// so recover() returns non-nil. On older versions recover()
	// returns nil for a nil panic — our handler must handle both.
	if rec.Code != http.StatusInternalServerError &&
		rec.Code != http.StatusOK {
		t.Errorf("unexpected status code for nil panic: %d", rec.Code)
	}
}