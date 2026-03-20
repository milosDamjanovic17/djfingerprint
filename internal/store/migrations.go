package store

import (
	"database/sql"
	"fmt"
)

// RunMigrations applies all schema migrations in order.
//
// Migrations are idempotent — each CREATE TABLE and CREATE INDEX
// statement uses IF NOT EXISTS so running migrations on a database
// that already has the schema is safe and produces no error.
//
// Migration order:
//
//	001 — songs table
//	002 — fingerprints table with foreign key to songs
//	003 — indexes for query performance
func RunMigrations(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("RunMigrations: db is nil")
	}

	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "001_create_songs",
			sql: `
				CREATE TABLE IF NOT EXISTS songs (
					id          SERIAL PRIMARY KEY,
					title       TEXT,
					artist      TEXT,
					album       TEXT,
					duration_ms INTEGER,
					source      TEXT        NOT NULL DEFAULT 'manual',
					spotify_id  TEXT,
					file_hash   TEXT        NOT NULL UNIQUE,
					created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
				)`,
		},
		{
			name: "002_create_fingerprints",
			sql: `
				CREATE TABLE IF NOT EXISTS fingerprints (
					id          BIGSERIAL PRIMARY KEY,
					hash        INTEGER     NOT NULL,
					song_id     INTEGER     NOT NULL
					            REFERENCES songs(id) ON DELETE CASCADE,
					anchor_time REAL        NOT NULL
				)`,
		},
		{
			name: "003_create_indexes",
			sql: `
				CREATE INDEX IF NOT EXISTS idx_fingerprints_hash
				    ON fingerprints (hash);

				CREATE INDEX IF NOT EXISTS idx_fingerprints_song_id
				    ON fingerprints (song_id);

				CREATE INDEX IF NOT EXISTS idx_songs_file_hash
				    ON songs (file_hash)`,
		},
	}

	for _, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.name, err)
		}
	}

	return nil
}