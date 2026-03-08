package store

import (
	"testing"
	"time"

	"github.com/milosDamjanovic17/djfingerprint/internal/dsp"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// makeHashes generates a slice of FingerprintHash values for testing.
// All hashes belong to the provided songID.
// Hash values are deterministic based on index so tests are repeatable.
func makeHashes(songID int, count int) []dsp.FingerprintHash {
	hashes := make([]dsp.FingerprintHash, count)
	for i := range hashes {
		hashes[i] = dsp.FingerprintHash{
			Hash:       int32(i + 1),
			SongID:     songID,
			AnchorTime: i * 10,
		}
	}
	return hashes
}

// insertTestSong inserts a throwaway song for use in fingerprint tests.
// Returns the inserted song ID.
// The caller is responsible for cleanup — deleting the song will
// cascade delete all associated fingerprint rows automatically.
func insertTestSong(t *testing.T, store *SongStore, suffix string) int {
	t.Helper()

	song := testSong("fptest" + suffix)
	inserted, err := store.Insert(song)
	if err != nil {
		t.Fatalf("insertTestSong: failed to insert song: %v", err)
	}

	return inserted.ID
}

// -----------------------------------------------------------------------
// InsertBatch — Input Validation
// -----------------------------------------------------------------------

func TestFingerprintStore_InsertBatch_EmptySlice_NoError(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	fs := NewFingerprintStore(db)

	// Empty slice must be a no-op — not an error
	err := fs.InsertBatch([]dsp.FingerprintHash{})
	if err != nil {
		t.Errorf("expected no error for empty hash slice, got: %v", err)
	}
}

// -----------------------------------------------------------------------
// InsertBatch — Row Count Verification
// -----------------------------------------------------------------------

func TestFingerprintStore_InsertBatch_CorrectRowCount(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "rowcount01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	const hashCount = 50
	hashes := makeHashes(songID, hashCount)

	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	count, err := fs.CountBySongID(songID)
	if err != nil {
		t.Fatalf("CountBySongID failed: %v", err)
	}

	if count != hashCount {
		t.Errorf("expected %d rows, got %d", hashCount, count)
	}
}

func TestFingerprintStore_InsertBatch_ExactlyOneBatch(t *testing.T) {
	// insertBatchSize rows = exactly one full batch
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "onebatch01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	hashes := makeHashes(songID, insertBatchSize)

	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	count, err := fs.CountBySongID(songID)
	if err != nil {
		t.Fatalf("CountBySongID failed: %v", err)
	}

	if count != insertBatchSize {
		t.Errorf("expected %d rows, got %d", insertBatchSize, count)
	}
}

func TestFingerprintStore_InsertBatch_BatchBoundary_101Rows(t *testing.T) {
	// 101 rows must split into 2 batches: 100 + 1
	// Both batches must complete successfully
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "boundary01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	const hashCount = insertBatchSize + 1
	hashes := makeHashes(songID, hashCount)

	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed at 101 rows: %v", err)
	}

	count, err := fs.CountBySongID(songID)
	if err != nil {
		t.Fatalf("CountBySongID failed: %v", err)
	}

	if count != hashCount {
		t.Errorf("expected %d rows after boundary batch, got %d", hashCount, count)
	}
}

func TestFingerprintStore_InsertBatch_BatchBoundary_199Rows(t *testing.T) {
	// 199 rows = 1 full batch of 100 + 1 partial batch of 99
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "boundary02")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	const hashCount = 199
	hashes := makeHashes(songID, hashCount)

	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed at 199 rows: %v", err)
	}

	count, err := fs.CountBySongID(songID)
	if err != nil {
		t.Fatalf("CountBySongID failed: %v", err)
	}

	if count != hashCount {
		t.Errorf("expected %d rows, got %d", hashCount, count)
	}
}

func TestFingerprintStore_InsertBatch_SingleRow(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "singlerow01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	hashes := makeHashes(songID, 1)

	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed for single row: %v", err)
	}

	count, err := fs.CountBySongID(songID)
	if err != nil {
		t.Fatalf("CountBySongID failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

// -----------------------------------------------------------------------
// InsertBatch — Data Integrity
// -----------------------------------------------------------------------

func TestFingerprintStore_InsertBatch_SongID_Correct(t *testing.T) {
	// Rows inserted for songID must not appear under a different songID
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	song1ID := insertTestSong(t, ss, "songid01a")
	song2ID := insertTestSong(t, ss, "songid01b")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, song1ID)
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, song2ID)

	// Insert 10 hashes for song1 and 20 for song2
	if err := fs.InsertBatch(makeHashes(song1ID, 10)); err != nil {
		t.Fatalf("InsertBatch song1 failed: %v", err)
	}
	if err := fs.InsertBatch(makeHashes(song2ID, 20)); err != nil {
		t.Fatalf("InsertBatch song2 failed: %v", err)
	}

	count1, err := fs.CountBySongID(song1ID)
	if err != nil {
		t.Fatalf("CountBySongID song1 failed: %v", err)
	}
	count2, err := fs.CountBySongID(song2ID)
	if err != nil {
		t.Fatalf("CountBySongID song2 failed: %v", err)
	}

	if count1 != 10 {
		t.Errorf("expected 10 rows for song1, got %d", count1)
	}
	if count2 != 20 {
		t.Errorf("expected 20 rows for song2, got %d", count2)
	}
}

func TestFingerprintStore_InsertBatch_TransactionRollback_OnFailure(t *testing.T) {
	// If we insert hashes for a non-existent song_id the foreign key
	// constraint must reject the insert and roll back the transaction.
	// No partial rows must remain in the database.
	db := testDB(t)
	defer db.Close()

	fs := NewFingerprintStore(db)

	// song_id 999999999 does not exist — FK constraint will fire
	hashes := makeHashes(999999999, 50)

	err := fs.InsertBatch(hashes)
	if err == nil {
		t.Fatal("expected error for non-existent song_id, got nil")
	}

	// Verify no rows were committed despite the partial attempt
	count, err := fs.CountBySongID(999999999)
	if err != nil {
		t.Fatalf("CountBySongID failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

// -----------------------------------------------------------------------
// QueryByHashes — Input Validation
// -----------------------------------------------------------------------

func TestFingerprintStore_QueryByHashes_EmptySlice_ReturnsEmpty(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	fs := NewFingerprintStore(db)

	results, err := fs.QueryByHashes([]dsp.FingerprintHash{})
	if err != nil {
		t.Fatalf("unexpected error for empty query: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected empty results for empty query, got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// QueryByHashes — Match Correctness
// -----------------------------------------------------------------------

func TestFingerprintStore_QueryByHashes_NoMatches_ReturnsEmpty(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	fs := NewFingerprintStore(db)

	// Query for hash values that are very unlikely to exist
	// Use large negative values outside normal hash range
	queryHashes := []dsp.FingerprintHash{
		{Hash: int32(-2000000000), SongID: 0, AnchorTime: 0},
		{Hash: int32(-1999999999), SongID: 0, AnchorTime: 0},
	}

	results, err := fs.QueryByHashes(queryHashes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for non-existent hashes, got %d", len(results))
	}
}

func TestFingerprintStore_QueryByHashes_ReturnsMatchingRows(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "querymatch01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	// Insert known hashes
	inserted := makeHashes(songID, 10)
	if err := fs.InsertBatch(inserted); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	// Query for the first 5 of the inserted hashes
	queryHashes := inserted[:5]

	results, err := fs.QueryByHashes(queryHashes)
	if err != nil {
		t.Fatalf("QueryByHashes failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results for known inserted hashes, got none")
	}

	// All results must belong to the correct song
	for _, r := range results {
		if r.SongID != songID {
			t.Errorf("expected SongID=%d, got %d", songID, r.SongID)
		}
	}
}

func TestFingerprintStore_QueryByHashes_SongID_InResults(t *testing.T) {
	// Insert hashes for two different songs.
	// Query only for song1 hashes.
	// Results must contain only song1's ID.
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	song1ID := insertTestSong(t, ss, "multisong01a")
	song2ID := insertTestSong(t, ss, "multisong01b")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, song1ID)
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, song2ID)

	// Give song1 and song2 non-overlapping hash values
	// song1: hash values 10001–10010
	// song2: hash values 20001–20010
	song1Hashes := make([]dsp.FingerprintHash, 10)
	for i := range song1Hashes {
		song1Hashes[i] = dsp.FingerprintHash{
			Hash:       int32(10001 + i),
			SongID:     song1ID,
			AnchorTime: i * 5,
		}
	}

	song2Hashes := make([]dsp.FingerprintHash, 10)
	for i := range song2Hashes {
		song2Hashes[i] = dsp.FingerprintHash{
			Hash:       int32(20001 + i),
			SongID:     song2ID,
			AnchorTime: i * 5,
		}
	}

	if err := fs.InsertBatch(song1Hashes); err != nil {
		t.Fatalf("InsertBatch song1 failed: %v", err)
	}
	if err := fs.InsertBatch(song2Hashes); err != nil {
		t.Fatalf("InsertBatch song2 failed: %v", err)
	}

	// Query only for song1 hash values
	results, err := fs.QueryByHashes(song1Hashes)
	if err != nil {
		t.Fatalf("QueryByHashes failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results for song1 hashes, got none")
	}

	for _, r := range results {
		if r.SongID != song1ID {
			t.Errorf(
				"expected only song1ID=%d in results, got SongID=%d",
				song1ID, r.SongID,
			)
		}
	}
}

func TestFingerprintStore_QueryByHashes_AnchorTime_Correct(t *testing.T) {
	// Verify that AnchorTime values are returned correctly
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "anchortime01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	// Insert one hash with a known anchor time
	knownHash := int32(777777)
	knownAnchorTime := 42

	hashes := []dsp.FingerprintHash{
		{Hash: knownHash, SongID: songID, AnchorTime: knownAnchorTime},
	}

	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	results, err := fs.QueryByHashes(hashes)
	if err != nil {
		t.Fatalf("QueryByHashes failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	found := false
	for _, r := range results {
		if r.Hash == knownHash && r.AnchorTime == knownAnchorTime {
			found = true
			break
		}
	}

	if !found {
		t.Errorf(
			"expected result with hash=%d anchorTime=%d, not found in results",
			knownHash, knownAnchorTime,
		)
	}
}

func TestFingerprintStore_QueryByHashes_HashValue_InResults(t *testing.T) {
	// Verify that the hash value itself is returned in results
	// The matcher uses hash values for coherence scoring
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "hashval01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	targetHash := int32(888888)
	hashes := []dsp.FingerprintHash{
		{Hash: targetHash, SongID: songID, AnchorTime: 10},
	}

	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	results, err := fs.QueryByHashes(hashes)
	if err != nil {
		t.Fatalf("QueryByHashes failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}

	found := false
	for _, r := range results {
		if r.Hash == targetHash {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected hash=%d in results, not found", targetHash)
	}
}

// -----------------------------------------------------------------------
// Performance Tests
// -----------------------------------------------------------------------

func TestFingerprintStore_InsertBatch_Performance_FullSong(t *testing.T) {
	// Insert a full song worth of fingerprints (~58,000 rows)
	// Must complete within 10 seconds
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "perf01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	// 58,000 rows — realistic for a 3-minute song
	const hashCount = 58000
	hashes := makeHashes(songID, hashCount)

	start := time.Now()
	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("InsertBatch %d rows took %v", hashCount, elapsed)

	if elapsed > 10*time.Second {
		t.Errorf(
			"InsertBatch too slow: %v (limit 10s) for %d rows",
			elapsed, hashCount,
		)
	}

	// Verify all rows were stored
	count, err := fs.CountBySongID(songID)
	if err != nil {
		t.Fatalf("CountBySongID failed: %v", err)
	}
	if count != hashCount {
		t.Errorf("expected %d rows after insert, got %d", hashCount, count)
	}
}

func TestFingerprintStore_QueryByHashes_Performance(t *testing.T) {
	// Insert a full song worth of hashes then query for 1500 of them.
	// The query must complete within 500ms using idx_fingerprints_hash.
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "queryperf01")
	defer db.Exec(`DELETE FROM songs WHERE id = $1`, songID)

	// Insert 58,000 rows
	const insertCount = 58000
	hashes := makeHashes(songID, insertCount)

	if err := fs.InsertBatch(hashes); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	// Query for 1500 hashes — realistic snippet size
	queryHashes := hashes[:1500]

	start := time.Now()
	results, err := fs.QueryByHashes(queryHashes)
	if err != nil {
		t.Fatalf("QueryByHashes failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("QueryByHashes %d hashes returned %d results in %v",
		len(queryHashes), len(results), elapsed,
	)

	if elapsed > 500*time.Millisecond {
		t.Errorf(
			"QueryByHashes too slow: %v (limit 500ms)",
			elapsed,
		)
	}

	if len(results) == 0 {
		t.Error("expected results for known inserted hashes")
	}
}

// -----------------------------------------------------------------------
// CascadeDelete — Fingerprints Removed With Song
// -----------------------------------------------------------------------

func TestFingerprintStore_CascadeDelete_RemovesFingerprints(t *testing.T) {
	// When a song is deleted all its fingerprints must be removed
	// automatically via ON DELETE CASCADE defined in Migration 002
	db := testDB(t)
	defer db.Close()

	ss := NewSongStore(db)
	fs := NewFingerprintStore(db)

	songID := insertTestSong(t, ss, "cascade01")

	// Insert fingerprints for the song
	if err := fs.InsertBatch(makeHashes(songID, 50)); err != nil {
		t.Fatalf("InsertBatch failed: %v", err)
	}

	// Verify fingerprints exist
	count, err := fs.CountBySongID(songID)
	if err != nil {
		t.Fatalf("CountBySongID failed: %v", err)
	}
	if count != 50 {
		t.Fatalf("expected 50 fingerprints before delete, got %d", count)
	}

	// Delete the song — cascade should remove fingerprints
	if err := ss.Delete(songID); err != nil {
		t.Fatalf("Delete song failed: %v", err)
	}

	// Fingerprints must be gone
	count, err = fs.CountBySongID(songID)
	if err != nil {
		t.Fatalf("CountBySongID after delete failed: %v", err)
	}
	if count != 0 {
		t.Errorf(
			"expected 0 fingerprints after song delete, got %d",
			count,
		)
	}
}