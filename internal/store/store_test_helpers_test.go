package store

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
)

// testDB returns a PostgreSQL connection for store tests.
// It reads connection details from environment variables with
// sensible defaults matching the .env file from Phase 2 setup.
//
// If the database is not available the test is skipped rather
// than failed — this allows the test suite to run in environments
// without PostgreSQL (e.g. pure DSP testing).
func testDB(t *testing.T) *sql.DB {
	t.Helper()

	host := envOrDefault("DB_HOST", "localhost")
	port := envOrDefault("DB_PORT", "5432")
	dbname := envOrDefault("DB_NAME", "djfingerprint")
	user := envOrDefault("DB_USER", "djapp")
	password := envOrDefault("DB_PASSWORD", "moose123")

	if password == "" {
		t.Skip("DB_PASSWORD not set — skipping PostgreSQL store test")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		host, port, dbname, user, password,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("could not open PostgreSQL connection: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Skipf("could not ping PostgreSQL: %v", err)
	}

	return db
}

// envOrDefault returns the value of an environment variable
// or the provided default when the variable is not set.
func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// testSong returns a minimal valid Song for use in store tests.
// The fileHash parameter must be unique per test to avoid
// unique constraint conflicts between tests.
func testSong(fileHash string) *models.Song {
	return &models.Song{
		Source:   "manual",
		FileHash: fileHash,
	}
}

// testSongWithMeta returns a Song with all metadata fields populated.
func testSongWithMeta(fileHash string) *models.Song {
	return &models.Song{
		Title:    models.StringPtr("Test Track"),
		Artist:   models.StringPtr("Test Artist"),
		Album:    models.StringPtr("Test Album"),
		Source:   "manual",
		FileHash: fileHash,
	}
}
