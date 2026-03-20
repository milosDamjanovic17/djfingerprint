package main

import (
	"context"
	"fmt"

	"github.com/milosDamjanovic17/djfingerprint/internal/dsp"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
	"github.com/milosDamjanovic17/djfingerprint/internal/store"
)

// -----------------------------------------------------------------------
// SongStoreAdapter
//
// Wraps *store.SongStore and satisfies handlers.SongStorer.
// Translates between the store's method signatures (no context, different
// names) and the interface the handlers expect.
// -----------------------------------------------------------------------

type SongStoreAdapter struct {
	s *store.SongStore
}

func NewSongStoreAdapter(s *store.SongStore) *SongStoreAdapter {
	return &SongStoreAdapter{s: s}
}

// CreateSong inserts a new song and populates ID, CreatedAt, UpdatedAt
// on the passed-in song from the RETURNING clause result.
func (a *SongStoreAdapter) CreateSong(_ context.Context, song *models.Song) error {
	result, err := a.s.Insert(song)
	if err != nil {
		return err
	}
	// Copy database-assigned fields back into the caller's struct
	song.ID = result.ID
	song.CreatedAt = result.CreatedAt
	song.UpdatedAt = result.UpdatedAt
	return nil
}

func (a *SongStoreAdapter) GetSongByHash(_ context.Context, hash string) (*models.Song, error) {
	return a.s.GetByFileHash(hash)
}

func (a *SongStoreAdapter) GetSongByID(_ context.Context, id int) (*models.Song, error) {
	return a.s.GetByID(id)
}

func (a *SongStoreAdapter) GetSongs(_ context.Context, page, pageSize int) ([]*models.Song, int, error) {
	return a.s.List(page, pageSize)
}

func (a *SongStoreAdapter) DeleteSong(_ context.Context, id int) error {
	return a.s.Delete(id)
}

// -----------------------------------------------------------------------
// FingerprintStoreAdapter
//
// Wraps *store.FingerprintStore and satisfies handlers.FingerprintStorer.
// Translates []models.Fingerprint ↔ []dsp.FingerprintHash and adds the
// DeleteHashesBySongID operation that the store does not yet have.
// -----------------------------------------------------------------------

type FingerprintStoreAdapter struct {
	s *store.FingerprintStore
}

func NewFingerprintStoreAdapter(s *store.FingerprintStore) *FingerprintStoreAdapter {
	return &FingerprintStoreAdapter{s: s}
}

// StoreFingerprints converts []models.Fingerprint to []dsp.FingerprintHash
// and delegates to the store's InsertBatch.
func (a *FingerprintStoreAdapter) StoreFingerprints(_ context.Context, fps []models.Fingerprint) error {
	hashes := make([]dsp.FingerprintHash, len(fps))
	for i, fp := range fps {
		hashes[i] = dsp.FingerprintHash{
			Hash:       fp.Hash,
			SongID:     fp.SongID,
			AnchorTime: fp.AnchorTime,
		}
	}
	return a.s.InsertBatch(hashes)
}

// QueryByHashes converts []int32 hash values into []dsp.FingerprintHash
// query objects and delegates to the store's QueryByHashes.
func (a *FingerprintStoreAdapter) QueryByHashes(_ context.Context, hashes []int32) ([]models.HashResult, error) {
	queryHashes := make([]dsp.FingerprintHash, len(hashes))
	for i, h := range hashes {
		queryHashes[i] = dsp.FingerprintHash{Hash: h}
	}
	return a.s.QueryByHashes(queryHashes)
}

// DeleteHashesBySongID removes all fingerprint rows for a given song ID.
// The store does not implement this directly — we execute it inline here
// using the store's underlying database connection via a raw DELETE.
func (a *FingerprintStoreAdapter) DeleteHashesBySongID(_ context.Context, songID int) error {
	db := a.s.DB()
	if db == nil {
		return fmt.Errorf("DeleteHashesBySongID: no database connection")
	}
	_, err := db.Exec(`DELETE FROM fingerprints WHERE song_id = $1`, songID)
	if err != nil {
		return fmt.Errorf("DeleteHashesBySongID: %w", err)
	}
	return nil
}