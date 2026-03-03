package store

import (
	"errors"
	"fmt"
	"testing"

	_ "github.com/milosDamjanovic17/djfingerprint/internal/models"
)

// -----------------------------------------------------------------------
// SongStore — Insert
// -----------------------------------------------------------------------

func TestSongStore_Insert_Success(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	store := &SongStore{db: db}
	song := testSong("aabbccdd001")

	result, err := store.Insert(song)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	if result.ID == 0 {
		t.Error("expected non-zero ID after insert")
	}
	if result.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt after insert")
	}
	if result.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt after insert")
	}
	if result.FileHash != song.FileHash {
		t.Errorf("expected FileHash=%q, got %q", song.FileHash, result.FileHash)
	}

	// Clean up
	tx.Rollback()
	db.Exec(`DELETE FROM songs WHERE file_hash = $1`, song.FileHash)
}

func TestSongStore_Insert_WithMetadata(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}
	song := testSongWithMeta("aabbccdd002")

	result, err := store.Insert(song)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, result.ID)

	if result.SongTitle() != "Test Track" {
		t.Errorf("expected Title='Test Track', got %q", result.SongTitle())
	}
	if result.SongArtist() != "Test Artist" {
		t.Errorf("expected Artist='Test Artist', got %q", result.SongArtist())
	}
	if result.SongAlbum() != "Test Album" {
		t.Errorf("expected Album='Test Album', got %q", result.SongAlbum())
	}
}

func TestSongStore_Insert_NullableFieldsNil(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}
	song := testSong("aabbccdd003")

	result, err := store.Insert(song)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, result.ID)

	if result.Title != nil {
		t.Error("expected Title=nil for song without title")
	}
	if result.Artist != nil {
		t.Error("expected Artist=nil for song without artist")
	}
	if result.Album != nil {
		t.Error("expected Album=nil for song without album")
	}
}

func TestSongStore_Insert_DuplicateHash_ReturnsDuplicateError(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}
	song := testSong("aabbccdd004")

	first, err := store.Insert(song)
	if err != nil {
		t.Fatalf("first Insert failed: %v", err)
	}
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, first.ID)

	// Second insert with same hash must return ErrDuplicateHash
	_, err = store.Insert(song)
	if err == nil {
		t.Fatal("expected error for duplicate file hash, got nil")
	}
	if !errors.Is(err, ErrDuplicateHash) {
		t.Errorf("expected ErrDuplicateHash, got %v", err)
	}
}

func TestSongStore_Insert_SourceDefaultManual(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}
	song := testSong("aabbccdd005")
	song.Source = "manual"

	result, err := store.Insert(song)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, result.ID)

	if result.Source != "manual" {
		t.Errorf("expected Source='manual', got %q", result.Source)
	}
}

// -----------------------------------------------------------------------
// SongStore — GetByID
// -----------------------------------------------------------------------

func TestSongStore_GetByID_Success(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}
	song := testSongWithMeta("aabbccdd006")

	inserted, err := store.Insert(song)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, inserted.ID)

	retrieved, err := store.GetByID(inserted.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.ID != inserted.ID {
		t.Errorf("expected ID=%d, got %d", inserted.ID, retrieved.ID)
	}
	if retrieved.SongTitle() != inserted.SongTitle() {
		t.Errorf("expected Title=%q, got %q", inserted.SongTitle(), retrieved.SongTitle())
	}
	if retrieved.FileHash != inserted.FileHash {
		t.Errorf("expected FileHash=%q, got %q", inserted.FileHash, retrieved.FileHash)
	}
}

func TestSongStore_GetByID_NotFound(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	_, err := store.GetByID(999999999)
	if err == nil {
		t.Fatal("expected ErrNotFound for nonexistent ID, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSongStore_GetByID_NullableFieldsRoundTrip(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	// Insert a song with no nullable fields set
	song := testSong("aabbccdd007")
	inserted, err := store.Insert(song)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, inserted.ID)

	retrieved, err := store.GetByID(inserted.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// All nullable fields must come back as nil
	if retrieved.Title != nil {
		t.Errorf("expected Title=nil, got %q", *retrieved.Title)
	}
	if retrieved.Artist != nil {
		t.Errorf("expected Artist=nil, got %q", *retrieved.Artist)
	}
	if retrieved.SpotifyID != nil {
		t.Errorf("expected SpotifyID=nil, got %q", *retrieved.SpotifyID)
	}
}

// -----------------------------------------------------------------------
// SongStore — GetByFileHash
// -----------------------------------------------------------------------

func TestSongStore_GetByFileHash_Success(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}
	song := testSong("aabbccdd008")

	inserted, err := store.Insert(song)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, inserted.ID)

	retrieved, err := store.GetByFileHash(song.FileHash)
	if err != nil {
		t.Fatalf("GetByFileHash failed: %v", err)
	}

	if retrieved.ID != inserted.ID {
		t.Errorf("expected ID=%d, got %d", inserted.ID, retrieved.ID)
	}
	if retrieved.FileHash != song.FileHash {
		t.Errorf("expected FileHash=%q, got %q", song.FileHash, retrieved.FileHash)
	}
}

func TestSongStore_GetByFileHash_NotFound(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	_, err := store.GetByFileHash("nonexistenthashvalue999")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// -----------------------------------------------------------------------
// SongStore — List
// -----------------------------------------------------------------------

func TestSongStore_List_Empty(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	// Use a fresh count — if the DB already has songs this test
	// just verifies List does not error on an empty page
	store := &SongStore{db: db}
	songs, total, err := store.List(1, 20)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// total and songs count must be consistent
	if len(songs) > total {
		t.Errorf("songs returned (%d) exceeds total count (%d)", len(songs), total)
	}
}

func TestSongStore_List_ReturnsInsertedSongs(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	// Insert 3 test songs
	insertedIDs := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		hash := fmt.Sprintf("listtest%08d", i)
		s, err := store.Insert(testSong(hash))
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		insertedIDs = append(insertedIDs, s.ID)
	}

	// Clean up after test
	defer func() {
		for _, id := range insertedIDs {
			db.Exec(`DELETE FROM songs WHERE id = $1`, id)
		}
	}()

	songs, total, err := store.List(1, 100)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if total < 3 {
		t.Errorf("expected total >= 3, got %d", total)
	}
	if len(songs) < 3 {
		t.Errorf("expected at least 3 songs returned, got %d", len(songs))
	}
}

func TestSongStore_List_Pagination(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	// Insert 5 songs
	insertedIDs := make([]int, 0, 5)
	for i := 0; i < 5; i++ {
		hash := fmt.Sprintf("pagetest%08d", i)
		s, err := store.Insert(testSong(hash))
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		insertedIDs = append(insertedIDs, s.ID)
	}

	defer func() {
		for _, id := range insertedIDs {
			db.Exec(`DELETE FROM songs WHERE id = $1`, id)
		}
	}()

	// Page 1 with pageSize=2 should return 2 songs
	page1, _, err := store.List(1, 2)
	if err != nil {
		t.Fatalf("List page 1 failed: %v", err)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 songs on page 1, got %d", len(page1))
	}

	// Page 2 with pageSize=2 should also return 2 songs
	page2, _, err := store.List(2, 2)
	if err != nil {
		t.Fatalf("List page 2 failed: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("expected 2 songs on page 2, got %d", len(page2))
	}

	// Pages must not contain the same song IDs
	page1IDs := make(map[int]bool)
	for _, s := range page1 {
		page1IDs[s.ID] = true
	}
	for _, s := range page2 {
		if page1IDs[s.ID] {
			t.Errorf("song ID %d appears on both page 1 and page 2", s.ID)
		}
	}
}

func TestSongStore_List_PageSizeCappedAt100(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	// Requesting more than 100 must be silently capped
	songs, _, err := store.List(1, 9999)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(songs) > 100 {
		t.Errorf("expected at most 100 songs, got %d", len(songs))
	}
}

func TestSongStore_List_InvalidPageDefaults(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	// Page 0 and negative page should not error
	_, _, err := store.List(0, 20)
	if err != nil {
		t.Errorf("List with page=0 should not error, got %v", err)
	}

	_, _, err = store.List(-1, 20)
	if err != nil {
		t.Errorf("List with page=-1 should not error, got %v", err)
	}
}

// -----------------------------------------------------------------------
// SongStore — Delete
// -----------------------------------------------------------------------

func TestSongStore_Delete_Success(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}
	song := testSong("aabbccdd009")

	inserted, err := store.Insert(song)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	if err := store.Delete(inserted.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify the song is gone
	_, err = store.GetByID(inserted.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestSongStore_Delete_NotFound(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	err := store.Delete(999999999)
	if err == nil {
		t.Fatal("expected ErrNotFound for nonexistent ID, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSongStore_Delete_RemovesOnlyTargetSong(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	store := &SongStore{db: db}

	// Insert two songs
	song1, err := store.Insert(testSong("aabbccdd010"))
	if err != nil {
		t.Fatalf("Insert song1 failed: %v", err)
	}

	song2, err := store.Insert(testSong("aabbccdd011"))
	if err != nil {
		t.Fatalf("Insert song2 failed: %v", err)
	}
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, song2.ID)

	// Delete only song1
	if err := store.Delete(song1.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Song2 must still exist
	retrieved, err := store.GetByID(song2.ID)
	if err != nil {
		t.Errorf("song2 should still exist after deleting song1, got error: %v", err)
	}
	if retrieved.ID != song2.ID {
		t.Errorf("expected song2 ID=%d, got %d", song2.ID, retrieved.ID)
	}
}