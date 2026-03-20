package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/milosDamjanovic17/djfingerprint/internal/models"
	"github.com/milosDamjanovic17/djfingerprint/internal/store"
)

// SongStorer defines the song storage operations required by the handlers.
type SongStorer interface {
	CreateSong(ctx context.Context, song *models.Song) error
	GetSongByHash(ctx context.Context, hash string) (*models.Song, error)
	GetSongByID(ctx context.Context, id int) (*models.Song, error)
	GetSongs(ctx context.Context, page, pageSize int) ([]*models.Song, int, error)
	DeleteSong(ctx context.Context, id int) error
}

// FingerprintStorer defines the fingerprint storage operations required
// by the handlers.
// Uses models.Fingerprint — the correct model type for this package.
type FingerprintStorer interface {
	StoreFingerprints(ctx context.Context, fingerprints []models.Fingerprint) error
	QueryByHashes(ctx context.Context, hashes []int32) ([]models.HashResult, error)
	DeleteHashesBySongID(ctx context.Context, songID int) error
}

// Handler holds the dependencies shared across all HTTP handlers.
type Handler struct {
	songStore        SongStorer
	fingerprintStore FingerprintStorer
}

// NewHandler creates a Handler with the given store implementations.
func NewHandler(songStore SongStorer, fingerprintStore FingerprintStorer) *Handler {
	return &Handler{
		songStore:        songStore,
		fingerprintStore: fingerprintStore,
	}
}

// writeJSON serialises v to JSON and writes it to w with the given
// HTTP status code and Content-Type: application/json header.
func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response with the given HTTP status
// code, machine-readable error code, and human-readable message.
func writeError(w http.ResponseWriter, status int, code, message string) {
	body := struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}{
		Error:   code,
		Message: message,
	}
	writeJSON(w, status, body) //nolint:errcheck
}

// getSongsByIDs retrieves song metadata for a slice of song IDs.
// Returns a map of songID → *models.Song.
// Song IDs that cannot be found are silently omitted.
func (h *Handler) getSongsByIDs(ctx context.Context, ids []int) map[int]*models.Song {
	result := make(map[int]*models.Song, len(ids))
	for _, id := range ids {
		song, err := h.songStore.GetSongByID(ctx, id)
		if err != nil {
			continue
		}
		result[id] = song
	}
	return result
}

// isNotFound returns true when the error indicates a record was not
// found in the store.
func isNotFound(err error) bool {
	return err == store.ErrNotFound
}