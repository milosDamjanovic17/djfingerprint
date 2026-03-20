package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// newTestRouter builds a chi router with the handler routes registered.
// Used by tests that need URL parameter extraction ({id}).
func newTestRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/songs", h.ListSongs)
	r.Get("/api/v1/songs/{id}", h.GetSong)
	r.Delete("/api/v1/songs/{id}", h.DeleteSong)
	return r
}

// -----------------------------------------------------------------------
// ListSongs
// -----------------------------------------------------------------------

func TestListSongs_EmptyLibrary_Returns200(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestListSongs_EmptyLibrary_ReturnsEmptySongsArray(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	songs, ok := body["songs"].([]interface{})
	if !ok {
		t.Fatal("expected songs array in response")
	}
	if len(songs) != 0 {
		t.Errorf("expected 0 songs, got %d", len(songs))
	}
}

func TestListSongs_WithSongs_ReturnsSongs(t *testing.T) {
	ss := newMockSongStore()
	ss.songs[1] = makeSong(1, "Track A", "Artist A")
	ss.songs[2] = makeSong(2, "Track B", "Artist B")
	ss.nextID = 3

	h := NewHandler(ss, newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	songs := body["songs"].([]interface{})
	if len(songs) != 2 {
		t.Errorf("expected 2 songs, got %d", len(songs))
	}
}

func TestListSongs_ContentTypeJSON(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestListSongs_Pagination_DefaultValues(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["page"].(float64) != float64(defaultPage) {
		t.Errorf("expected page=%d, got %v", defaultPage, body["page"])
	}
	if body["page_size"].(float64) != float64(defaultPageSize) {
		t.Errorf("expected page_size=%d, got %v", defaultPageSize, body["page_size"])
	}
}

func TestListSongs_Pagination_CustomValues(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs?page=2&page_size=10", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["page"].(float64) != 2 {
		t.Errorf("expected page=2, got %v", body["page"])
	}
	if body["page_size"].(float64) != 10 {
		t.Errorf("expected page_size=10, got %v", body["page_size"])
	}
}

func TestListSongs_PageSizeExceedsMax_ClampsToDefault(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/v1/songs?page_size=%d", maxPageSize+1),
		nil,
	)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["page_size"].(float64) != float64(defaultPageSize) {
		t.Errorf("expected page_size clamped to %d, got %v", defaultPageSize, body["page_size"])
	}
}

func TestListSongs_StoreError_Returns500(t *testing.T) {
	ss := newMockSongStore()
	ss.failOn = "GetSongs"

	h := NewHandler(ss, newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// GetSong
// -----------------------------------------------------------------------

func TestGetSong_ValidID_Returns200(t *testing.T) {
	ss := newMockSongStore()
	ss.songs[1] = makeSong(1, "Knock Knock", "DJ Koze")

	h := NewHandler(ss, newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGetSong_ValidID_ReturnsCorrectSong(t *testing.T) {
	ss := newMockSongStore()
	ss.songs[1] = makeSong(1, "Knock Knock", "DJ Koze")

	h := NewHandler(ss, newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["id"].(float64) != 1 {
		t.Errorf("expected id=1, got %v", body["id"])
	}
}

func TestGetSong_InvalidID_Returns400(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestGetSong_NotFound_Returns404(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGetSong_NotFound_ReturnsErrorCode(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["error"] != "not_found" {
		t.Errorf("expected error='not_found', got %v", body["error"])
	}
}

func TestGetSong_StoreError_Returns500(t *testing.T) {
	ss := newMockSongStore()
	ss.failOn = "GetSongByID"

	h := NewHandler(ss, newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestGetSong_ZeroID_Returns400(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/songs/0", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for id=0, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// DeleteSong
// -----------------------------------------------------------------------

func TestDeleteSong_ValidID_Returns204(t *testing.T) {
	ss := newMockSongStore()
	ss.songs[1] = makeSong(1, "Track", "Artist")

	h := NewHandler(ss, newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestDeleteSong_ValidID_RemovesSong(t *testing.T) {
	ss := newMockSongStore()
	ss.songs[1] = makeSong(1, "Track", "Artist")

	h := NewHandler(ss, newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Song must be gone from the store
	if _, ok := ss.songs[1]; ok {
		t.Error("expected song to be removed from store after DELETE")
	}
}

func TestDeleteSong_InvalidID_Returns400(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/xyz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestDeleteSong_NotFound_Returns404(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDeleteSong_FingerprintStoreError_Returns500(t *testing.T) {
	ss := newMockSongStore()
	ss.songs[1] = makeSong(1, "Track", "Artist")

	fs := newMockFingerprintStore()
	fs.failOn = "DeleteHashesBySongID"

	h := NewHandler(ss, fs)
	r := newTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestDeleteSong_SongStoreError_Returns500(t *testing.T) {
	ss := newMockSongStore()
	ss.songs[1] = makeSong(1, "Track", "Artist")

	h := NewHandler(ss, newMockFingerprintStore())
	r := newTestRouter(h)

	// Trigger DeleteSong failure after the existence check passes
	ss.failOn = "DeleteSong"

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/songs/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// parseIntParam
// -----------------------------------------------------------------------

func TestParseIntParam_ValidValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?page=3", nil)
	result := parseIntParam(req, "page", 1)
	if result != 3 {
		t.Errorf("expected 3, got %d", result)
	}
}

func TestParseIntParam_MissingParam_ReturnsDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	result := parseIntParam(req, "page", 1)
	if result != 1 {
		t.Errorf("expected default 1, got %d", result)
	}
}

func TestParseIntParam_InvalidValue_ReturnsDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?page=abc", nil)
	result := parseIntParam(req, "page", 1)
	if result != 1 {
		t.Errorf("expected default 1 for invalid value, got %d", result)
	}
}