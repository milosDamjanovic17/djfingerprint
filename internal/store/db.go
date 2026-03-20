package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Open opens a PostgreSQL connection pool using the provided DSN,
// verifies the connection with a ping, and configures pool limits.
//
// The returned *sql.DB is safe for concurrent use. It must be closed
// when the application exits — main.go defers db.Close().
//
// Connection pool settings:
//   - MaxOpenConns: 25  — maximum simultaneous connections to PostgreSQL
//   - MaxIdleConns: 25  — connections kept open when not in use
//   - ConnMaxLifetime: 5m — connections recycled after 5 minutes
//
// These values are appropriate for a single-instance API server.
// They prevent connection exhaustion on the PostgreSQL side while
// keeping enough connections ready to handle burst traffic.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("store.Open: failed to open database: %w", err)
	}

	// Configure the connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify the connection is actually reachable
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store.Open: database ping failed: %w", err)
	}

	return db, nil
}