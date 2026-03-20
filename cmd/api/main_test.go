package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"net"

	"github.com/milosDamjanovic17/djfingerprint/internal/api"
	"github.com/milosDamjanovic17/djfingerprint/internal/api/handlers"
	"github.com/milosDamjanovic17/djfingerprint/internal/config"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
	"github.com/milosDamjanovic17/djfingerprint/internal/store"
)

// -----------------------------------------------------------------------
// Stub stores — satisfy handler interfaces without a database
// -----------------------------------------------------------------------

type mainStubSongStore struct{}

func (s *mainStubSongStore) CreateSong(_ context.Context, _ *models.Song) error {
	return nil
}
func (s *mainStubSongStore) GetSongByHash(_ context.Context, _ string) (*models.Song, error) {
	return nil, store.ErrNotFound
}
func (s *mainStubSongStore) GetSongByID(_ context.Context, _ int) (*models.Song, error) {
	return nil, store.ErrNotFound
}
func (s *mainStubSongStore) GetSongs(_ context.Context, _, _ int) ([]*models.Song, int, error) {
	return []*models.Song{}, 0, nil
}
func (s *mainStubSongStore) DeleteSong(_ context.Context, _ int) error {
	return store.ErrNotFound
}

type mainStubFingerprintStore struct{}

func (s *mainStubFingerprintStore) StoreFingerprints(_ context.Context, _ []models.Fingerprint) error {
	return nil
}
func (s *mainStubFingerprintStore) QueryByHashes(_ context.Context, _ []int32) ([]models.HashResult, error) {
	return []models.HashResult{}, nil
}
func (s *mainStubFingerprintStore) DeleteHashesBySongID(_ context.Context, _ int) error {
	return nil
}

// buildTestRouter assembles a router with stub stores — mirrors what
// main() does but without a real database connection.
func buildTestRouter() http.Handler {
	h := handlers.NewHandler(
		&mainStubSongStore{},
		&mainStubFingerprintStore{},
	)
	return api.NewRouter(h)
}

// -----------------------------------------------------------------------
// config.Load — verify defaults are sensible without env vars set
// -----------------------------------------------------------------------

func TestMain_ConfigLoad_ReturnsNonNilConfig(t *testing.T) {
	// config.Load reads .env — it may fail in CI without DB_PASSWORD.
	// We only assert it does not panic and returns a typed result.
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config.Load requires environment variables not set in this context: %v", err)
	}
	if cfg == nil {
		t.Fatal("config.Load() returned nil config")
	}
}

func TestMain_ConfigLoad_AppPortIsSet(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config.Load requires environment: %v", err)
	}
	if cfg.AppPort == "" {
		t.Error("expected non-empty AppPort")
	}
}

func TestMain_ConfigLoad_DSNPostgres_IsNonEmpty(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config.Load requires environment: %v", err)
	}
	dsn := cfg.DSNPostgres()
	if dsn == "" {
		t.Error("expected non-empty DSN from DSNPostgres()")
	}
}

func TestMain_ConfigLoad_DSNPostgres_ContainsHost(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("config.Load requires environment: %v", err)
	}
	dsn := cfg.DSNPostgres()
	if len(dsn) == 0 {
		t.Fatal("DSN is empty")
	}
	// DSN must contain "host=" — produced by DSNPostgres()
	found := false
	for i := 0; i <= len(dsn)-5; i++ {
		if dsn[i:i+5] == "host=" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DSN does not contain 'host=': %q", dsn)
	}
}

// -----------------------------------------------------------------------
// Adapter wiring — verify adapters satisfy handler interfaces
// -----------------------------------------------------------------------

func TestMain_SongStoreAdapter_ImplementsInterface(t *testing.T) {
	// Compile-time check — if SongStoreAdapter does not satisfy
	// handlers.SongStorer the build fails before this test runs.
	// The runtime test confirms NewSongStoreAdapter returns non-nil.
	adapter := NewSongStoreAdapter(store.NewSongStore(nil))
	if adapter == nil {
		t.Error("NewSongStoreAdapter returned nil")
	}
}

func TestMain_FingerprintStoreAdapter_ImplementsInterface(t *testing.T) {
	adapter := NewFingerprintStoreAdapter(store.NewFingerprintStore(nil))
	if adapter == nil {
		t.Error("NewFingerprintStoreAdapter returned nil")
	}
}

// -----------------------------------------------------------------------
// Router wiring — assembled router serves requests correctly
// -----------------------------------------------------------------------

func TestMain_Router_ListSongs_Returns200(t *testing.T) {
	ts := httptest.NewServer(buildTestRouter())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/songs")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestMain_Router_UnknownRoute_Returns404(t *testing.T) {
	ts := httptest.NewServer(buildTestRouter())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestMain_Router_PostUpload_RouteExists(t *testing.T) {
	ts := httptest.NewServer(buildTestRouter())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/upload", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// 400 — route matched, handler rejected the missing body
	if resp.StatusCode == http.StatusNotFound {
		t.Error("POST /api/v1/upload returned 404 — route not registered")
	}
	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.Error("POST /api/v1/upload returned 405 — wrong method registered")
	}
}

func TestMain_Router_PostIdentify_RouteExists(t *testing.T) {
	ts := httptest.NewServer(buildTestRouter())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/identify", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Error("POST /api/v1/identify returned 404 — route not registered")
	}
	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.Error("POST /api/v1/identify returned 405 — wrong method registered")
	}
}

func TestMain_Router_GetSongByID_RouteExists(t *testing.T) {
	ts := httptest.NewServer(buildTestRouter())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/songs/1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// 404 here is from the handler (stub returns ErrNotFound), not the router
	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.Error("GET /api/v1/songs/{id} returned 405 — wrong method registered")
	}
}

func TestMain_Router_DeleteSong_RouteExists(t *testing.T) {
	ts := httptest.NewServer(buildTestRouter())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/songs/1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMethodNotAllowed {
		t.Error("DELETE /api/v1/songs/{id} returned 405 — wrong method registered")
	}
}

func TestMain_Router_ResponseIsJSON(t *testing.T) {
	ts := httptest.NewServer(buildTestRouter())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/songs")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

// -----------------------------------------------------------------------
// Graceful shutdown — context timeout plumbing works correctly
// -----------------------------------------------------------------------

func TestMain_GracefulShutdown_CompletesWithinTimeout(t *testing.T) {
	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: buildTestRouter(),
	}

	go srv.ListenAndServe() //nolint:errcheck
	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("graceful shutdown failed: %v", err)
	}
}

func TestMain_GracefulShutdown_AfterShutdown_ServerRejectsNewRequests(t *testing.T) {
	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: buildTestRouter(),
	}

	// Start on a random port
	listener, err := newListener()
	if err != nil {
		t.Fatalf("could not create listener: %v", err)
	}
	addr := listener.Addr().String()

	go srv.Serve(listener) //nolint:errcheck
	time.Sleep(10 * time.Millisecond)

	// Shut down
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx) //nolint:errcheck

	// New requests should now fail — server is closed
	_, err = http.Get("http://" + addr + "/api/v1/songs")
	if err == nil {
		t.Error("expected connection error after shutdown, got nil")
	}
}

func newListener() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}