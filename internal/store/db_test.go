package store_test

import (
	"testing"

	"github.com/milosDamjanovic17/djfingerprint/internal/store"
)

// Open with an invalid DSN — should fail at Ping, not panic
func TestOpen_InvalidDSN_ReturnsError(t *testing.T) {
	db, err := store.Open("host=invalid port=0 dbname=none user=none password=none sslmode=disable")
	if err == nil {
		db.Close()
		t.Error("expected error for invalid DSN, got nil")
	}
}

func TestOpen_EmptyDSN_ReturnsError(t *testing.T) {
	db, err := store.Open("")
	if err == nil {
		db.Close()
		t.Error("expected error for empty DSN, got nil")
	}
}