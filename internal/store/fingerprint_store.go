package store

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
	"github.com/milosDamjanovic17/djfingerprint/internal/dsp"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
)

const (
	// insertBatchSize is the number of fingerprint rows inserted
	// per SQL statement.
	//
	// PostgreSQL hard limit: 65535 parameters per statement.
	// Each fingerprint row uses 3 parameters (hash, song_id, anchor_time).
	// Maximum safe rows = floor(65535 / 3) = 21845.
	//
	// We use 100 — well within the limit, efficient enough to reduce
	// 58,000 individual inserts down to ~580 batched statements.
	insertBatchSize = 100
)

// FingerprintStore handles all database operations for the
// fingerprints table.
//
// Both operations on this store are performance critical:
//   - InsertBatch: ~58,000 rows per song upload
//   - QueryByHashes: runs on every identification request
type FingerprintStore struct {
	db *sql.DB
}

// NewFingerprintStore creates a new FingerprintStore with the provided
// database connection pool.
func NewFingerprintStore(db *sql.DB) *FingerprintStore {
	return &FingerprintStore{db: db}
}

// InsertBatch inserts all fingerprint hashes for a song into the
// fingerprints table using batched multi-row INSERT statements.
//
// A single 3-minute song generates approximately 58,000 hash rows.
// Inserting them one by one would require 58,000 round trips to
// PostgreSQL. Batching groups rows into chunks of insertBatchSize
// reducing round trips to approximately 580.
//
// All batches are wrapped in a single transaction — either all
// hashes are inserted or none are. A partial fingerprint set
// would produce incorrect match results.
func (fs *FingerprintStore) InsertBatch(hashes []dsp.FingerprintHash) error {
	if len(hashes) == 0 {
		return nil
	}

	// Wrap all batch inserts in a single transaction.
	// If any batch fails the entire set is rolled back.
	// A partial fingerprint set is worse than no fingerprints —
	// it would produce low-confidence incorrect matches.
	tx, err := fs.db.Begin()
	if err != nil {
		return fmt.Errorf("fingerprint insert batch: failed to begin transaction: %w", err)
	}

	// Ensure rollback on any failure path.
	// If Commit() succeeds, the deferred Rollback() is a no-op.
	defer tx.Rollback()

	// Process hashes in chunks of insertBatchSize
	for i := 0; i < len(hashes); i += insertBatchSize {
		end := i + insertBatchSize
		if end > len(hashes) {
			end = len(hashes)
		}

		batch := hashes[i:end]

		if err := insertBatch(tx, batch); err != nil {
			return fmt.Errorf(
				"fingerprint insert batch: failed at offset %d: %w",
				i, err,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("fingerprint insert batch: commit failed: %w", err)
	}

	return nil
}

// insertBatch executes a single multi-row INSERT for one batch of hashes.
// The SQL and parameter slice are built dynamically because the number
// of rows (and therefore parameters) varies per batch.
//
// Example for a batch of 2 rows:
//
//	INSERT INTO fingerprints (hash, song_id, anchor_time)
//	VALUES ($1, $2, $3), ($4, $5, $6)
func insertBatch(tx *sql.Tx, batch []dsp.FingerprintHash) error {
	if len(batch) == 0 {
		return nil
	}

	// Build the VALUES clause dynamically.
	// Each row contributes one "($ n, $ n+1, $ n+2)" placeholder group.
	valueStrings := make([]string, 0, len(batch))
	valueArgs := make([]interface{}, 0, len(batch)*3)

	for i, h := range batch {
		// Parameter numbers are 1-indexed and sequential across
		// the entire statement — not reset per row.
		base := i * 3
		valueStrings = append(valueStrings,
			fmt.Sprintf("($%d, $%d, $%d)", base+1, base+2, base+3),
		)
		valueArgs = append(valueArgs, h.Hash, h.SongID, h.AnchorTime)
	}

	query := fmt.Sprintf(
		"INSERT INTO fingerprints (hash, song_id, anchor_time) VALUES %s",
		strings.Join(valueStrings, ", "),
	)

	_, err := tx.Exec(query, valueArgs...)
	if err != nil {
		return fmt.Errorf("insertBatch exec failed: %w", err)
	}

	return nil
}

// QueryByHashes finds all fingerprint rows whose hash value matches
// any hash in the provided query slice.
//
// This is the matching query — it runs on every identification request.
// It uses PostgreSQL's ANY($1) syntax with pq.Array() to pass the
// entire hash slice as a single array parameter, hitting
// idx_fingerprints_hash directly.
//
// Returns an empty slice (not an error) when no matches are found —
// "no match" is a valid identification outcome.
func (fs *FingerprintStore) QueryByHashes(
	hashes []dsp.FingerprintHash,
) ([]models.HashResult, error) {
	if len(hashes) == 0 {
		return []models.HashResult{}, nil
	}

	// Extract just the hash values into a []int32 slice for pq.Array
	hashValues := make([]int32, len(hashes))
	for i, h := range hashes {
		hashValues[i] = h.Hash
	}

	query := `
		SELECT song_id, anchor_time, hash
		FROM fingerprints
		WHERE hash = ANY($1)`

	rows, err := fs.db.Query(query, pq.Array(hashValues))
	if err != nil {
		return nil, fmt.Errorf("fingerprint query by hashes failed: %w", err)
	}
	defer rows.Close()

	results := make([]models.HashResult, 0)

	for rows.Next() {
		var hr models.HashResult
		if err := rows.Scan(&hr.SongID, &hr.AnchorTime, &hr.Hash); err != nil {
			return nil, fmt.Errorf("fingerprint query scan failed: %w", err)
		}
		results = append(results, hr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fingerprint query rows error: %w", err)
	}

	return results, nil
}

// CountBySongID returns the number of fingerprint rows stored for
// a given song ID.
// Used in tests to verify InsertBatch stored the correct number of rows.
func (fs *FingerprintStore) CountBySongID(songID int) (int, error) {
	var count int
	err := fs.db.QueryRow(
		`SELECT COUNT(*) FROM fingerprints WHERE song_id = $1`,
		songID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("fingerprint count by song id failed: %w", err)
	}
	return count, nil
}

// DB returns the underlying *sql.DB connection pool.
// Used by adapter layers that need to execute queries not covered
// by the store's own methods.
func (fs *FingerprintStore) DB() *sql.DB {
	return fs.db
}