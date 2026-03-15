package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// makeRequest is a helper that sends a request through the CORS
// middleware and returns the recorded response.
func makeRequest(method, path, origin string) *httptest.ResponseRecorder {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	handler.ServeHTTP(rec, req)
	return rec
}

// -----------------------------------------------------------------------
// CORS Headers — Allowed Origins
// -----------------------------------------------------------------------

func TestCORS_AllowedOrigin_HeaderPresent(t *testing.T) {
	rec := makeRequest(http.MethodGet, "/", "http://localhost:5173")

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:5173" {
		t.Errorf(
			"expected Access-Control-Allow-Origin=http://localhost:5173, got %q",
			origin,
		)
	}
}

func TestCORS_AllowedOrigin_MethodsHeaderPresent(t *testing.T) {
	rec := makeRequest(http.MethodGet, "/", "http://localhost:5173")

	methods := rec.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("expected Access-Control-Allow-Methods header to be present")
	}
}

func TestCORS_AllowedOrigin_HeadersHeaderPresent(t *testing.T) {
	rec := makeRequest(http.MethodGet, "/", "http://localhost:5173")

	headers := rec.Header().Get("Access-Control-Allow-Headers")
	if headers == "" {
		t.Error("expected Access-Control-Allow-Headers header to be present")
	}
}

func TestCORS_AllowedOrigin_MaxAgePresent(t *testing.T) {
	rec := makeRequest(http.MethodGet, "/", "http://localhost:5173")

	maxAge := rec.Header().Get("Access-Control-Max-Age")
	if maxAge == "" {
		t.Error("expected Access-Control-Max-Age header to be present")
	}
}

func TestCORS_Port3000_AllowedOrigin(t *testing.T) {
	rec := makeRequest(http.MethodGet, "/", "http://localhost:3000")

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf(
			"expected Access-Control-Allow-Origin=http://localhost:3000, got %q",
			origin,
		)
	}
}

func TestCORS_NoOriginHeader_PassesThrough(t *testing.T) {
	// Requests with no Origin header (non-browser, curl, etc.)
	// must pass through to the handler without CORS headers
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Origin header set

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for no-origin request, got %d", rec.Code)
	}
}

func TestCORS_DisallowedOrigin_NoAllowHeader(t *testing.T) {
	rec := makeRequest(http.MethodGet, "/", "http://evil.com")

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin == "http://evil.com" {
		t.Error("disallowed origin must not receive Access-Control-Allow-Origin header")
	}
}

// -----------------------------------------------------------------------
// OPTIONS Preflight
// -----------------------------------------------------------------------

func TestCORS_Options_Returns200(t *testing.T) {
	rec := makeRequest(http.MethodOptions, "/api/v1/upload", "http://localhost:5173")

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS preflight, got %d", rec.Code)
	}
}

func TestCORS_Options_DoesNotReachHandler(t *testing.T) {
	handlerCalled := false

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/upload", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	handler.ServeHTTP(rec, req)

	if handlerCalled {
		t.Error("OPTIONS preflight must not reach the downstream handler")
	}
}

func TestCORS_Options_HasCORSHeaders(t *testing.T) {
	rec := makeRequest(http.MethodOptions, "/api/v1/upload", "http://localhost:5173")

	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("OPTIONS response must include Access-Control-Allow-Methods")
	}
	if rec.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("OPTIONS response must include Access-Control-Allow-Headers")
	}
}

// -----------------------------------------------------------------------
// Non-OPTIONS Requests Pass Through
// -----------------------------------------------------------------------

func TestCORS_Get_ReachesHandler(t *testing.T) {
	handlerCalled := false

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("GET request must reach the downstream handler")
	}
}

func TestCORS_Post_ReachesHandler(t *testing.T) {
	handlerCalled := false

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("POST request must reach the downstream handler")
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201 from handler, got %d", rec.Code)
	}
}

func TestCORS_Delete_ReachesHandler(t *testing.T) {
	handlerCalled := false

	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("DELETE request must reach the downstream handler")
	}
}

// -----------------------------------------------------------------------
// isAllowedOrigin
// -----------------------------------------------------------------------

func TestIsAllowedOrigin_KnownOrigins(t *testing.T) {
	allowed := []string{
		"http://localhost:5173",
		"http://localhost:3000",
		"http://localhost:8080",
	}

	for _, origin := range allowed {
		if !isAllowedOrigin(origin) {
			t.Errorf("expected %q to be an allowed origin", origin)
		}
	}
}

func TestIsAllowedOrigin_UnknownOrigin(t *testing.T) {
	unknown := []string{
		"http://evil.com",
		"https://localhost:5173", // https vs http
		"http://localhost:9999",  // wrong port
		"",
	}

	for _, origin := range unknown {
		if isAllowedOrigin(origin) {
			t.Errorf("expected %q to NOT be an allowed origin", origin)
		}
	}
}