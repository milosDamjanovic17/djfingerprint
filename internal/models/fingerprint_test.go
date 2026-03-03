package models

import (
	"encoding/json"
	"math"
	"testing"
)

// -----------------------------------------------------------------------
// Fingerprint — Construction
// -----------------------------------------------------------------------

func TestFingerprint_ZeroValue_NoPanic(t *testing.T) {
	var f Fingerprint
	_ = f.ID
	_ = f.Hash
	_ = f.SongID
	_ = f.AnchorTime
}

func TestNewFingerprint_FieldsSet(t *testing.T) {
	fp := NewFingerprint(123456, 42, 100)

	if fp.Hash != 123456 {
		t.Errorf("expected Hash=123456, got %d", fp.Hash)
	}
	if fp.SongID != 42 {
		t.Errorf("expected SongID=42, got %d", fp.SongID)
	}
	if fp.AnchorTime != 100 {
		t.Errorf("expected AnchorTime=100, got %d", fp.AnchorTime)
	}
}

func TestNewFingerprint_IDIsZero(t *testing.T) {
	// ID is assigned by PostgreSQL BIGSERIAL on INSERT
	// It must be zero before the row is stored
	fp := NewFingerprint(123456, 42, 100)
	if fp.ID != 0 {
		t.Errorf("expected ID=0 before insert, got %d", fp.ID)
	}
}

// -----------------------------------------------------------------------
// Fingerprint — Hash Field Range
// -----------------------------------------------------------------------

func TestFingerprint_Hash_MaxInt32(t *testing.T) {
	fp := &Fingerprint{Hash: MaxInt32Hash}
	if fp.Hash != math.MaxInt32 {
		t.Errorf("expected Hash=MaxInt32 (%d), got %d", math.MaxInt32, fp.Hash)
	}
}

func TestFingerprint_Hash_MinInt32(t *testing.T) {
	fp := &Fingerprint{Hash: MinInt32Hash}
	if fp.Hash != math.MinInt32 {
		t.Errorf("expected Hash=MinInt32 (%d), got %d", math.MinInt32, fp.Hash)
	}
}

func TestFingerprint_Hash_Zero(t *testing.T) {
	fp := &Fingerprint{Hash: 0}
	if fp.Hash != 0 {
		t.Errorf("expected Hash=0, got %d", fp.Hash)
	}
}

// -----------------------------------------------------------------------
// Fingerprint — ID Field is int64
// -----------------------------------------------------------------------

func TestFingerprint_ID_LargeValue(t *testing.T) {
	// BIGSERIAL can produce values larger than int32 max
	// Verify the int64 field holds large values without overflow
	const largeID = int64(5000000000) // 5 billion — exceeds int32 max
	fp := &Fingerprint{ID: largeID}
	if fp.ID != largeID {
		t.Errorf("expected ID=%d, got %d", largeID, fp.ID)
	}
}

// -----------------------------------------------------------------------
// HashResult — Construction
// -----------------------------------------------------------------------

func TestHashResult_ZeroValue_NoPanic(t *testing.T) {
	var hr HashResult
	_ = hr.SongID
	_ = hr.AnchorTime
	_ = hr.Hash
}

func TestHashResult_Fields(t *testing.T) {
	hr := HashResult{
		SongID:     7,
		AnchorTime: 150,
		Hash:       int32(999999),
	}

	if hr.SongID != 7 {
		t.Errorf("expected SongID=7, got %d", hr.SongID)
	}
	if hr.AnchorTime != 150 {
		t.Errorf("expected AnchorTime=150, got %d", hr.AnchorTime)
	}
	if hr.Hash != int32(999999) {
		t.Errorf("expected Hash=999999, got %d", hr.Hash)
	}
}

// -----------------------------------------------------------------------
// Fingerprint — JSON
// -----------------------------------------------------------------------

func TestFingerprint_JSON_FieldNames(t *testing.T) {
	fp := Fingerprint{
		ID:         1,
		Hash:       123456,
		SongID:     42,
		AnchorTime: 100,
	}

	data, err := json.Marshal(fp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expectedKeys := []string{"id", "hash", "song_id", "anchor_time"}
	for _, key := range expectedKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q not found", key)
		}
	}
}

func TestFingerprint_JSON_RoundTrip(t *testing.T) {
	original := Fingerprint{
		ID:         999,
		Hash:       int32(-123456),
		SongID:     7,
		AnchorTime: 42,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Fingerprint
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Hash != original.Hash {
		t.Errorf("Hash mismatch: %d vs %d", decoded.Hash, original.Hash)
	}
	if decoded.SongID != original.SongID {
		t.Errorf("SongID mismatch: %d vs %d", decoded.SongID, original.SongID)
	}
	if decoded.AnchorTime != original.AnchorTime {
		t.Errorf("AnchorTime mismatch: %d vs %d", decoded.AnchorTime, original.AnchorTime)
	}
}