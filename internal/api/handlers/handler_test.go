package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/milosDamjanovic17/djfingerprint/internal/models"
	"github.com/milosDamjanovic17/djfingerprint/internal/store"
)

// -----------------------------------------------------------------------
// mockSongStore — in-memory SongStorer implementation for tests
// -----------------------------------------------------------------------

type mockSongStore struct {
	mu     sync.Mutex
	songs  map[int]*models.Song
	nextID int
	failOn string // method name to fail on — simulates DB errors
}

func newMockSongStore() *mockSongStore {
	return &mockSongStore{
		songs:  make(map[int]*models.Song),
		nextID: 1,
	}
}

func (m *mockSongStore) CreateSong(_ context.Context, song *models.Song) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn == "CreateSong" {
		return errMock
	}
	song.ID = m.nextID
	m.nextID++
	song.CreatedAt = time.Now()
	song.UpdatedAt = time.Now()
	copy := *song
	m.songs[song.ID] = &copy
	return nil
}

func (m *mockSongStore) GetSongByHash(_ context.Context, hash string) (*models.Song, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn == "GetSongByHash" {
		return nil, errMock
	}
	for _, s := range m.songs {
		if s.FileHash == hash {
			return s, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *mockSongStore) GetSongByID(_ context.Context, id int) (*models.Song, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn == "GetSongByID" {
		return nil, errMock
	}
	s, ok := m.songs[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return s, nil
}

func (m *mockSongStore) GetSongs(_ context.Context, page, pageSize int) ([]*models.Song, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn == "GetSongs" {
		return nil, 0, errMock
	}

	all := make([]*models.Song, 0, len(m.songs))
	for _, s := range m.songs {
		all = append(all, s)
	}

	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		return []*models.Song{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (m *mockSongStore) DeleteSong(_ context.Context, id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn == "DeleteSong" {
		return errMock
	}
	if _, ok := m.songs[id]; !ok {
		return store.ErrNotFound
	}
	delete(m.songs, id)
	return nil
}

// -----------------------------------------------------------------------
// mockFingerprintStore — in-memory FingerprintStorer for tests
// -----------------------------------------------------------------------

type mockFingerprintStore struct {
	mu           sync.Mutex
	fingerprints []models.Fingerprint
	results      []models.HashResult
	failOn       string
}

func newMockFingerprintStore() *mockFingerprintStore {
	return &mockFingerprintStore{}
}

// StoreFingerprints implements FingerprintStorer.
func (m *mockFingerprintStore) StoreFingerprints(_ context.Context, fingerprints []models.Fingerprint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn == "StoreFingerprints" {
		return errMock
	}
	m.fingerprints = append(m.fingerprints, fingerprints...)
	return nil
}

// QueryByHashes implements FingerprintStorer.
func (m *mockFingerprintStore) QueryByHashes(_ context.Context, _ []int32) ([]models.HashResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn == "QueryByHashes" {
		return nil, errMock
	}
	return m.results, nil
}

// DeleteHashesBySongID implements FingerprintStorer.
func (m *mockFingerprintStore) DeleteHashesBySongID(_ context.Context, songID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOn == "DeleteHashesBySongID" {
		return errMock
	}
	filtered := m.fingerprints[:0]
	for _, fp := range m.fingerprints {
		if fp.SongID != songID {
			filtered = append(filtered, fp)
		}
	}
	m.fingerprints = filtered
	return nil
}

// -----------------------------------------------------------------------
// Shared test helpers
// -----------------------------------------------------------------------

// errMock is the sentinel error returned by mock stores when failOn is set.
var errMock = fmt.Errorf("mock store error")

// makeSong builds a models.Song for use in tests.
func makeSong(id int, title, artist string) *models.Song {
	return &models.Song{
		ID:        id,
		Title:     models.StringPtr(title),
		Artist:    models.StringPtr(artist),
		Source:    "manual",
		FileHash:  fmt.Sprintf("hash-%d", id),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}