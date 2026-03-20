package store_test

import (
	"testing"

	"github.com/milosDamjanovic17/djfingerprint/internal/store"
)

// RunMigrations with a nil db — should return an error, not panic
func TestRunMigrations_NilDB_ReturnsError(t *testing.T) {
	err := store.RunMigrations(nil)
	if err == nil {
		t.Error("expected error for nil db, got nil")
	}
}