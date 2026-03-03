package models

// Fingerprint represents a single hash row in the fingerprints table.
//
// The fingerprints table is the largest in the application —
// a single 3-minute song generates approximately 58,000 rows.
// This struct is kept as small as possible to minimise memory
// pressure when large result sets are loaded during matching.
//
// Field sizes are chosen to match PostgreSQL column types exactly:
//
//	ID         → BIGSERIAL (64-bit) → int64
//	Hash       → INTEGER   (32-bit signed) → int32
//	SongID     → INTEGER   (32-bit signed) → int
//	AnchorTime → INTEGER   (32-bit signed) → int
type Fingerprint struct {
	ID         int64 `json:"id"`
	Hash       int32 `json:"hash"`
	SongID     int   `json:"song_id"`
	AnchorTime int   `json:"anchor_time"`
}

// HashResult is a lightweight struct used during the matching query.
// When the store queries fingerprints by hash values it does not need
// the full Fingerprint — it only needs the song ID and anchor time
// to build the coherence scoring map in the matcher.
//
// Using a dedicated struct rather than Fingerprint avoids scanning
// the ID column unnecessarily in the hot matching query path.
type HashResult struct {
	SongID     int   `json:"song_id"`
	AnchorTime int   `json:"anchor_time"`
	Hash       int32 `json:"hash"`
}

// NewFingerprint creates a Fingerprint from the fields produced by
// the DSP engine. ID is left as zero — it is assigned by PostgreSQL
// on INSERT via BIGSERIAL.
func NewFingerprint(hash int32, songID int, anchorTime int) *Fingerprint {
	return &Fingerprint{
		Hash:       hash,
		SongID:     songID,
		AnchorTime: anchorTime,
	}
}

// MaxInt32Hash is the maximum value a 32-bit signed hash can hold.
// Used in range validation tests.
const MaxInt32Hash = int32(2147483647)

// MinInt32Hash is the minimum value a 32-bit signed hash can hold.
// The hash encoding uses bitwise OR on positive values so in practice
// hashes will not be negative, but the field type supports it.
const MinInt32Hash = int32(-2147483648)