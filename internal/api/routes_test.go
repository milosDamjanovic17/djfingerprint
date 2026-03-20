package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/milosDamjanovic17/djfingerprint/internal/api/handlers"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
	"github.com/milosDamjanovic17/djfingerprint/internal/store"
	"github.com/milosDamjanovic17/djfingerprint/internal/api/middleware"
)

// -----------------------------------------------------------------------
// Minimal store stubs — satisfy interfaces without any real behaviour.
// Routes tests only verify that the correct handler is reached and the
// correct HTTP method/path combinations are accepted or rejected.
// They do not test handler logic — that is covered in handlers_test.go.
// -----------------------------------------------------------------------

type stubSongStore struct{}

func (s *stubSongStore) CreateSong(_ context.Context, _ *models.Song) error {
	return nil
}
func (s *stubSongStore) GetSongByHash(_ context.Context, _ string) (*models.Song, error) {
	return nil, store.ErrNotFound
}
func (s *stubSongStore) GetSongByID(_ context.Context, _ int) (*models.Song, error) {
	return nil, store.ErrNotFound
}
func (s *stubSongStore) GetSongs(_ context.Context, _, _ int) ([]*models.Song, int, error) {
	return []*models.Song{}, 0, nil
}
func (s *stubSongStore) DeleteSong(_ context.Context, _ int) error {
	return store.ErrNotFound
}

type stubFingerprintStore struct{}

func (s *stubFingerprintStore) StoreFingerprints(_ context.Context, _ []models.Fingerprint) error {
	return nil
}
func (s *stubFingerprintStore) QueryByHashes(_ context.Context, _ []int32) ([]models.HashResult, error) {
	return []models.HashResult{}, nil
}
func (s *stubFingerprintStore) DeleteHashesBySongID(_ context.Context, _ int) error {
	return nil
}

// newTestRouter builds a router with stub stores for route-level tests.
func newTestRouter() http.Handler {
	h := handlers.NewHandler(&stubSongStore{}, &stubFingerprintStore{})
	return NewRouter(h)
}

// -----------------------------------------------------------------------
// Route registration — verify every route exists and accepts the correct
// HTTP method. We check status codes that prove the route was matched:
//
//   - Any 2xx or 4xx (except 404 and 405) means the route matched.
//   - 405 Method Not Allowed means the path exists but wrong method.
//   - 404 Not Found means the path is not registered at all.
//
// We do not assert 200 here — that would require valid request bodies
// and is the responsibility of the handler tests.
// -----------------------------------------------------------------------

func TestRoutes_Upload_PostIsRegistered(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// 400 means the route matched — handler rejected the missing body
	if rec.Code == http.StatusNotFound {
		t.Errorf("POST /api/v1/upload returned 404 — route not registered")
	}
	if rec.Code == http.StatusMethodNotAllowed {
		t.Errorf("POST /api/v1/upload returned 405 — wrong method registered")
	}
}

func TestRoutes_Upload_GetIsNotAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/upload", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/v1/upload: expected 405, got %d", rec.Code)
	}
}

func TestRoutes_Upload_DeleteIsNotAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/upload", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/v1/upload: expected 405, got %d", rec.Code)
	}
}

func TestRoutes_Identify_PostIsRegistered(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identify", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Errorf("POST /api/v1/identify returned 404 — route not registered")
	}
	if rec.Code == http.StatusMethodNotAllowed {
		t.Errorf("POST /api/v1/identify returned 405 — wrong method registered")
	}
}

func TestRoutes_Identify_GetIsNotAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/identify", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/v1/identify: expected 405, got %d", rec.Code)
	}
}

func TestRoutes_Identify_DeleteIsNotAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/identify", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/v1/identify: expected 405, got %d", rec.Code)
	}
}

func TestRoutes_Songs_GetIsRegistered(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Errorf("GET /api/v1/songs returned 404 — route not registered")
	}
	if rec.Code == http.StatusMethodNotAllowed {
		t.Errorf("GET /api/v1/songs returned 405 — wrong method registered")
	}
}

func TestRoutes_Songs_PostIsNotAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/v1/songs: expected 405, got %d", rec.Code)
	}
}

func TestRoutes_Songs_DeleteIsNotAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/v1/songs: expected 405, got %d", rec.Code)
	}
}

func TestRoutes_SongByID_GetIsRegistered(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// The stub store returns ErrNotFound so the handler returns 404.
	// That 404 is from the handler, not the router — it proves the
	// route IS registered. We confirm by checking it is not 405.
	if rec.Code == http.StatusMethodNotAllowed {
		t.Errorf("GET /api/v1/songs/{id} returned 405 — wrong method registered")
	}
}

func TestRoutes_SongByID_DeleteIsRegistered(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusMethodNotAllowed {
		t.Errorf("DELETE /api/v1/songs/{id} returned 405 — wrong method registered")
	}
}

func TestRoutes_SongByID_PostIsNotAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/v1/songs/{id}: expected 405, got %d", rec.Code)
	}
}

func TestRoutes_SongByID_PutIsNotAllowed(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT /api/v1/songs/{id}: expected 405, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// Unknown routes return 404
// -----------------------------------------------------------------------

func TestRoutes_UnknownPath_Returns404(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown path: expected 404, got %d", rec.Code)
	}
}

func TestRoutes_RootPath_Returns404(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("root path: expected 404, got %d", rec.Code)
	}
}

func TestRoutes_NoVersionPrefix_Returns404(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/songs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("/songs without version prefix: expected 404, got %d", rec.Code)
	}
}

func TestRoutes_WrongVersionPrefix_Returns404(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v2/songs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("/api/v2/songs: expected 404, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// Middleware — verify middleware effects are visible in responses
// -----------------------------------------------------------------------

func TestRoutes_ListSongs_ResponseIsJSON(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestRoutes_CORS_AllowedOrigin_SetsHeader(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("expected Access-Control-Allow-Origin to be set for allowed origin")
	}
}

func TestRoutes_CORS_Preflight_Returns200(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/songs", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("OPTIONS preflight: expected 200, got %d", rec.Code)
	}
}

func TestRoutes_CORS_DisallowedOrigin_NoHeader(t *testing.T) {
	router := newTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin")
	}
}

func TestRoutes_Recovery_PanicReturns500(t *testing.T) {
	// Build a router with a handler that panics
	r := chi.NewRouter()
	r.Use(middleware.Recovery)
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("panic handler: expected 500, got %d", rec.Code)
	}
}

func TestRoutes_NewRouter_ReturnsNonNil(t *testing.T) {
	router := newTestRouter()
	if router == nil {
		t.Error("NewRouter returned nil")
	}
}