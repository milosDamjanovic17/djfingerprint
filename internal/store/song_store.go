package store

import (
	"database/sql"
	"errors"
	"fmt"
	_ "time"

	"github.com/milosDamjanovic17/djfingerprint/internal/models"
	_ "github.com/lib/pq"
)

// ErrNotFound is returned by store methods when a requested record
// does not exist in the database.
// Callers check for this specific error to return HTTP 404 responses
// rather than HTTP 500 responses.
var ErrNotFound = errors.New("record not found")

// ErrDuplicateHash is returned by Insert when a song with the same
// file_hash already exists in the database.
// The file_hash unique constraint in Migration 001 enforces this at
// the database level — this error surfaces it cleanly to the caller.
var ErrDuplicateHash = errors.New("song with this file hash already exists")

// SongStore handles all database operations for the songs table.
// It holds a reference to the shared *sql.DB connection pool.
// The connection pool is opened once in main.go and injected here.
type SongStore struct {
	db *sql.DB
}

// NewSongStore creates a new SongStore with the provided database
// connection pool. The pool must already be open and verified
// before this function is called.
func NewSongStore(db *sql.DB) *SongStore {
	return &SongStore{db: db}
}

// Insert persists a new song to the songs table.
//
// The file_hash field must be unique — if a song with the same hash
// already exists, ErrDuplicateHash is returned so the caller can
// return HTTP 409 without treating it as a server error.
//
// The RETURNING clause populates the ID, CreatedAt and UpdatedAt
// fields on the returned song — these are assigned by PostgreSQL
// and are not known until after the INSERT completes.
func (s *SongStore) Insert(song *models.Song) (*models.Song, error) {
	query := `
		INSERT INTO songs (
			title,
			artist,
			album,
			duration_ms,
			source,
			spotify_id,
			file_hash
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	row := s.db.QueryRow(
		query,
		nullableString(song.Title),
		nullableString(song.Artist),
		nullableString(song.Album),
		nullableInt(song.DurationMs),
		song.Source,
		nullableString(song.SpotifyID),
		song.FileHash,
	)

	result := *song
	err := row.Scan(&result.ID, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		// Check for PostgreSQL unique constraint violation
		// pq error code 23505 = unique_violation
		if isUniqueViolation(err) {
			return nil, ErrDuplicateHash
		}
		return nil, fmt.Errorf("song insert failed: %w", err)
	}

	return &result, nil
}

// GetByID retrieves a single song by its primary key.
// Returns ErrNotFound when no song exists with the given ID.
func (s *SongStore) GetByID(id int) (*models.Song, error) {
	query := `
		SELECT
			id,
			title,
			artist,
			album,
			duration_ms,
			source,
			spotify_id,
			file_hash,
			created_at,
			updated_at
		FROM songs
		WHERE id = $1`

	row := s.db.QueryRow(query, id)

	song, err := scanSong(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("song get by id failed: %w", err)
	}

	return song, nil
}

// GetByFileHash retrieves a single song by its SHA256 file hash.
// Returns ErrNotFound when no song exists with the given hash.
// Called during upload to detect duplicate files before fingerprinting.
func (s *SongStore) GetByFileHash(hash string) (*models.Song, error) {
	query := `
		SELECT
			id,
			title,
			artist,
			album,
			duration_ms,
			source,
			spotify_id,
			file_hash,
			created_at,
			updated_at
		FROM songs
		WHERE file_hash = $1`

	row := s.db.QueryRow(query, hash)

	song, err := scanSong(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("song get by file hash failed: %w", err)
	}

	return song, nil
}

// List retrieves a paginated list of songs ordered by created_at descending.
// Returns the songs for the requested page, the total song count, and any error.
//
// page is 1-indexed — page 1 returns the first pageSize songs.
// pageSize is capped at 100 to prevent excessively large result sets.
func (s *SongStore) List(page, pageSize int) ([]*models.Song, int, error) {
	// Enforce sensible bounds
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Get total count first — needed for pagination metadata in the response
	var total int
	countQuery := `SELECT COUNT(*) FROM songs`
	if err := s.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("song list count failed: %w", err)
	}

	// No songs yet — return empty slice rather than running the list query
	if total == 0 {
		return []*models.Song{}, 0, nil
	}

	offset := (page - 1) * pageSize

	listQuery := `
		SELECT
			id,
			title,
			artist,
			album,
			duration_ms,
			source,
			spotify_id,
			file_hash,
			created_at,
			updated_at
		FROM songs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(listQuery, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("song list query failed: %w", err)
	}
	defer rows.Close()

	songs := make([]*models.Song, 0, pageSize)
	for rows.Next() {
		song, err := scanSongRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("song list scan failed: %w", err)
		}
		songs = append(songs, song)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("song list rows error: %w", err)
	}

	return songs, total, nil
}

// Delete removes a song from the songs table by its primary key.
// The ON DELETE CASCADE constraint defined in Migration 002 ensures
// all associated fingerprint rows are removed automatically.
// Returns ErrNotFound when no song exists with the given ID.
func (s *SongStore) Delete(id int) error {
	query := `DELETE FROM songs WHERE id = $1`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("song delete failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("song delete rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// -----------------------------------------------------------------------
// Private helpers
// -----------------------------------------------------------------------

// scanSong scans a *sql.Row (single row result from QueryRow)
// into a *models.Song. Used by GetByID and GetByFileHash.
func scanSong(row *sql.Row) (*models.Song, error) {
	var song models.Song
	var title, artist, album, spotifyID sql.NullString
	var durationMs sql.NullInt64

	err := row.Scan(
		&song.ID,
		&title,
		&artist,
		&album,
		&durationMs,
		&song.Source,
		&spotifyID,
		&song.FileHash,
		&song.CreatedAt,
		&song.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	song.Title = toStringPtr(title)
	song.Artist = toStringPtr(artist)
	song.Album = toStringPtr(album)
	song.SpotifyID = toStringPtr(spotifyID)
	song.DurationMs = toIntPtr(durationMs)

	return &song, nil
}

// scanSongRow scans a *sql.Rows (multi-row result from Query)
// into a *models.Song. Used by List.
func scanSongRow(rows *sql.Rows) (*models.Song, error) {
	var song models.Song
	var title, artist, album, spotifyID sql.NullString
	var durationMs sql.NullInt64

	err := rows.Scan(
		&song.ID,
		&title,
		&artist,
		&album,
		&durationMs,
		&song.Source,
		&spotifyID,
		&song.FileHash,
		&song.CreatedAt,
		&song.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	song.Title = toStringPtr(title)
	song.Artist = toStringPtr(artist)
	song.Album = toStringPtr(album)
	song.SpotifyID = toStringPtr(spotifyID)
	song.DurationMs = toIntPtr(durationMs)

	return &song, nil
}

// nullableString converts a *string to a sql.NullString for use
// in query parameters. nil pointer becomes a NULL database value.
func nullableString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

// nullableInt converts a *int to a sql.NullInt64 for use
// in query parameters. nil pointer becomes a NULL database value.
func nullableInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

// toStringPtr converts a sql.NullString from a scan result back
// to a *string for the models.Song struct.
// NULL database values become nil pointers.
func toStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// toIntPtr converts a sql.NullInt64 from a scan result back
// to a *int for the models.Song struct.
func toIntPtr(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

// isUniqueViolation checks whether a PostgreSQL error is a unique
// constraint violation (error code 23505).
// We check the error string directly since the pq.Error type
// requires a type assertion that adds complexity.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL unique violation error code is 23505
	// The pq driver includes this in the error message
	return contains(err.Error(), "23505") ||
		contains(err.Error(), "unique constraint") ||
		contains(err.Error(), "unique_violation")
}

// contains is a simple substring check helper.
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			findSubstring(s, substr))
}

// findSubstring searches for substr within s.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}