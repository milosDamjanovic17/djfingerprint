# Phase 4 — Step 1 — `models/` Clarification Document

---

## What the Models Package Is

The `models` package is the **shared data language of the entire application**. It contains nothing but pure Go structs that mirror the three database tables defined in Phase 1. No logic, no database calls, no HTTP concerns, no imports from our own packages.

Every other layer in the application speaks in terms of these structs:

```
store/      → reads and writes models to/from PostgreSQL
matcher/    → receives models, returns models
handlers/   → converts models to/from JSON via dto/
dto/        → wraps models for HTTP transport
```

---

## Why a Separate Models Package

Without a dedicated models package, structs get defined wherever they are first needed and then either duplicated or imported in awkward directions. A single `models/` package gives every layer a clean, shared vocabulary with no circular imports.

```
models has NO imports from our codebase
      ↑
      │ imported by
      ├── store/
      ├── matcher/
      ├── api/handlers/
      └── api/dto/
```

---

## The Three Files

### `models/song.go`

Mirrors the `songs` table exactly. Represents a single indexed track in the DJ library.

**Database table reminder from Phase 1:**
```sql
CREATE TABLE songs (
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255),
    artist      VARCHAR(255),
    album       VARCHAR(255),
    duration_ms INTEGER,
    source      VARCHAR(50) NOT NULL DEFAULT 'manual',
    spotify_id  VARCHAR(100),
    file_hash   CHAR(64) NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**What the struct will look like:**
```
Song
  ├── ID          int
  ├── Title       string (nullable → *string)
  ├── Artist      string (nullable → *string)
  ├── Album       string (nullable → *string)
  ├── DurationMs  int    (nullable → *int)
  ├── Source      string
  ├── SpotifyID   string (nullable → *string)
  ├── FileHash    string
  ├── CreatedAt   time.Time
  └── UpdatedAt   time.Time
```

**Why pointer types for nullable fields:**
`title`, `artist`, `album`, `duration_ms`, and `spotify_id` are all nullable in the database. A track uploaded from an unlabeled CD file may have none of these. Using pointer types (`*string`, `*int`) rather than plain `string` and `int` means:
- A `nil` pointer correctly represents a database `NULL`
- An empty string `""` and a missing value `NULL` are not conflated
- The store layer can use `sql.NullString` internally and convert to `*string` cleanly

---

### `models/fingerprint.go`

Mirrors the `fingerprints` table. Represents a single hash row.

**Database table reminder from Phase 1:**
```sql
CREATE TABLE fingerprints (
    id          BIGSERIAL PRIMARY KEY,
    hash        INTEGER NOT NULL,
    song_id     INTEGER NOT NULL,
    anchor_time INTEGER NOT NULL
);
```

**What the struct will look like:**
```
Fingerprint
  ├── ID         int64   (BIGSERIAL → int64)
  ├── Hash       int32   (INTEGER signed → int32, matches encodeHash output)
  ├── SongID     int
  └── AnchorTime int
```

**Why `int64` for ID:**
The `fingerprints` table uses `BIGSERIAL` because it will grow to tens or hundreds of millions of rows. `BIGSERIAL` is a 64-bit integer in PostgreSQL — it must be `int64` in Go, not `int32` or plain `int`.

**Why `int32` for Hash:**
The hash is produced by `encodeHash` in `dsp/fingerprint.go` which returns `int32`. PostgreSQL `INTEGER` is also 32-bit signed. The types match exactly across Go and the database with no conversion needed.

---

### `models/song_feature.go`

Mirrors the `song_features` table. Represents the Spotify audio features for a track. All feature fields are nullable — they are only populated for tracks with a known Spotify ID.

**Database table reminder from Phase 1:**
```sql
CREATE TABLE song_features (
    id               SERIAL PRIMARY KEY,
    song_id          INTEGER NOT NULL,
    spotify_track_id VARCHAR(100),
    energy           FLOAT,
    danceability     FLOAT,
    valence          FLOAT,
    acousticness     FLOAT,
    instrumentalness FLOAT,
    liveness         FLOAT,
    speechiness      FLOAT,
    loudness         FLOAT,
    tempo            FLOAT,
    musical_key      INTEGER,
    created_at       TIMESTAMP NOT NULL DEFAULT NOW()
);
```

**What the struct will look like:**
```
SongFeature
  ├── ID               int
  ├── SongID           int
  ├── SpotifyTrackID   *string
  ├── Energy           *float64
  ├── Danceability     *float64
  ├── Valence          *float64
  ├── Acousticness     *float64
  ├── Instrumentalness *float64
  ├── Liveness         *float64
  ├── Speechiness      *float64
  ├── Loudness         *float64
  ├── Tempo            *float64
  ├── MusicalKey       *int
  └── CreatedAt        time.Time
```

**Why all feature fields are pointers:**
Every audio feature column in the database is nullable. A track without a Spotify ID has no features at all. Using pointer types means the presence or absence of each feature can be checked with a simple `nil` check — no magic sentinel values like `-1` or `0.0` are needed.

---

## JSON Tags Strategy

All three structs will carry JSON tags. This matters because:

- The `dto/` layer will embed or reference these structs when building API responses
- JSON field names use `snake_case` to match the database column names and to be consistent with what the Vue frontend will expect
- Nullable pointer fields use `omitempty` so `null` fields are omitted from JSON output rather than appearing as `null` — cleaner API responses

```go
// Example of the tagging approach
type Song struct {
    ID        int     `json:"id"`
    Title     *string `json:"title,omitempty"`
    Artist    *string `json:"artist,omitempty"`
    FileHash  string  `json:"file_hash"`
}
```

---

## What the Models Package Does NOT Contain

These things do not belong in `models/` and will not be added:

| Concern | Where It Lives |
|---------|---------------|
| SQL queries | `store/` |
| Validation logic | `api/handlers/` |
| HTTP serialization | `api/dto/` |
| Business rules | `store/` or `matcher/` |
| Default value assignment | `store/` |
| Pointer helper functions | Each model file, small helpers only |

---

## Helper Functions

Each model file will contain one small helper to make working with nullable pointer fields less verbose in the layers above:

```go
// Example — avoids nil pointer dereference when reading optional fields
func StringVal(s *string) string {
    if s == nil {
        return ""
    }
    return *s
}
```

These helpers will be defined per-type in each model file and are the only logic the models package will contain.

---

## What the Tests Will Cover

Since models are pure data structs, the tests focus on:

```
[ ] JSON marshaling produces correct snake_case field names
[ ] Nullable pointer fields marshal to omitempty correctly
    (nil pointer fields absent from JSON output)
[ ] Non-nil pointer fields appear correctly in JSON output
[ ] Zero value structs are valid (no panics on empty instantiation)
[ ] Helper functions return correct values for nil and non-nil inputs
[ ] int32 hash field accepts the full signed 32-bit range
[ ] time.Time fields marshal to RFC3339 format
```

---

## Summary

```
models/song.go          → Song struct, 10 fields, nullable title/artist/album
models/fingerprint.go   → Fingerprint struct, 4 fields, int32 hash, int64 id
models/song_feature.go  → SongFeature struct, 14 fields, all features nullable
```

No logic. No SQL. No HTTP. Just the shared data structures that every other
layer in Phase 4 depends on.

---

# `models/` — Build Summary

---

## What We Built

### The Purpose
The `models` package is the **shared data language of the entire application**. It contains three files — one per database table — each holding a pure Go struct that mirrors the corresponding PostgreSQL table defined in Phase 1. No logic beyond simple helper methods, no SQL, no HTTP, no imports from our own packages.

Every other layer in Phase 4 speaks in terms of these structs:

```
models/ has zero imports from our own codebase
            ↑
            │ imported by
            ├── store/
            ├── matcher/
            ├── api/handlers/
            └── api/dto/
```

---

## `models/song.go` — What Was Built and Why

### The `Song` Struct

```go
type Song struct {
    ID         int       `json:"id"`
    Title      *string   `json:"title,omitempty"`
    Artist     *string   `json:"artist,omitempty"`
    Album      *string   `json:"album,omitempty"`
    DurationMs *int      `json:"duration_ms,omitempty"`
    Source     string    `json:"source"`
    SpotifyID  *string   `json:"spotify_id,omitempty"`
    FileHash   string    `json:"file_hash"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

**Why pointer types for nullable fields:**

`Title`, `Artist`, `Album`, `DurationMs`, and `SpotifyID` are all nullable in the database. A track uploaded from an unlabeled CD has none of these. Using pointer types (`*string`, `*int`) rather than plain value types means:

| Scenario | Plain `string` | `*string` |
|----------|---------------|-----------|
| Field not known | `""` — ambiguous | `nil` — unambiguous |
| Field is empty string | `""` — same as above | `&""` — distinct |
| Field has value | `"DJ Koze"` | `&"DJ Koze"` |

A `nil` pointer maps correctly to a database `NULL`. An empty string and a missing value are never conflated.

**Why `omitempty` on nullable fields:**
Nil pointer fields are omitted from JSON output entirely rather than appearing as `null`. This produces cleaner API responses — a track with no artist simply has no `artist` key in the JSON rather than `"artist": null`.

---

### Methods on `Song`

| Method | Returns | Purpose |
|--------|---------|---------|
| `SongTitle()` | `string` | Safe dereference of `*Title`, returns `""` if nil |
| `SongArtist()` | `string` | Safe dereference of `*Artist`, returns `""` if nil |
| `SongAlbum()` | `string` | Safe dereference of `*Album`, returns `""` if nil |
| `HasSpotifyID()` | `bool` | True when SpotifyID is non-nil and non-empty |
| `IsManualUpload()` | `bool` | True when Source is `"manual"` |
| `DisplayName()` | `string` | Most descriptive name available, four fallback levels |

**`DisplayName()` fallback chain:**

```
Artist + Title both known  →  "DJ Koze - Knock Knock"
Title only                 →  "Knock Knock"
Artist only                →  "DJ Koze"
Neither known              →  "Unknown Track"
```

This is the key feature for the DJ use case — unlabeled white label tracks show as `"Unknown Track"` rather than crashing or showing empty strings.

---

### Package-Level Functions on `Song`

**`NewSong(fileHash string) *Song`**
Constructor that creates a `Song` with `Source="manual"` and the file hash populated. All nullable fields left `nil` — filled in by the upload handler after metadata extraction.

**`IsValidSource(source string) bool`**
Validates a source value against the three permitted values (`manual`, `spotify`, `unknown`). Mirrors the `CHECK` constraint defined in Migration 001. Case sensitive — `"Manual"` and `"MANUAL"` are rejected.

**`stringVal(s *string) string`**
Private helper. Safe dereference of a `*string`. Returns `""` for nil. Used by all three `Song` getter methods.

---

## `models/fingerprint.go` — What Was Built and Why

### The `Fingerprint` Struct

```go
type Fingerprint struct {
    ID         int64 `json:"id"`
    Hash       int32 `json:"hash"`
    SongID     int   `json:"song_id"`
    AnchorTime int   `json:"anchor_time"`
}
```

**Why `int64` for `ID`:**
The `fingerprints` table uses `BIGSERIAL` because it will grow to tens or hundreds of millions of rows for a reasonably sized DJ library. `BIGSERIAL` is a 64-bit signed integer in PostgreSQL. Using `int32` or plain `int` would overflow above ~2.1 billion rows. `int64` supports up to 9.2 quintillion — more than sufficient.

**Why `int32` for `Hash`:**
The hash is produced by `encodeHash` in `dsp/fingerprint.go` which explicitly returns `int32`. PostgreSQL `INTEGER` is also a 32-bit signed integer. The types match exactly across Go and the database with no conversion or casting needed at any layer.

---

### The `HashResult` Struct

```go
type HashResult struct {
    SongID     int   `json:"song_id"`
    AnchorTime int   `json:"anchor_time"`
    Hash       int32 `json:"hash"`
}
```

A lightweight companion to `Fingerprint` used exclusively during the matching query path. When `fingerprint_store.go` queries the database by hash values during identification, it does not need the row `ID` — only `SongID`, `AnchorTime`, and `Hash` are needed to build the coherence scoring map in the matcher.

Scanning only three columns instead of four removes unnecessary I/O on the single most performance-critical query in the application.

---

### Constants on `Fingerprint`

```go
const MaxInt32Hash = int32(2147483647)
const MinInt32Hash = int32(-2147483648)
```

Used in range validation tests to verify the `Hash` field correctly holds the full signed 32-bit range. In practice, hashes produced by the DSP engine are always positive (they are built from bitwise OR on positive values), but the field type supports the full range.

---

### `NewFingerprint(hash int32, songID int, anchorTime int) *Fingerprint`

Constructor. Creates a `Fingerprint` with `ID=0` — the ID is assigned by PostgreSQL `BIGSERIAL` on `INSERT` and is not known until after the row is written.

---

## `models/song_feature.go` — What Was Built and Why

### The `SongFeature` Struct

```go
type SongFeature struct {
    ID               int      `json:"id"`
    SongID           int      `json:"song_id"`
    SpotifyTrackID   *string  `json:"spotify_track_id,omitempty"`
    Energy           *float64 `json:"energy,omitempty"`
    Danceability     *float64 `json:"danceability,omitempty"`
    Valence          *float64 `json:"valence,omitempty"`
    Acousticness     *float64 `json:"acousticness,omitempty"`
    Instrumentalness *float64 `json:"instrumentalness,omitempty"`
    Liveness         *float64 `json:"liveness,omitempty"`
    Speechiness      *float64 `json:"speechiness,omitempty"`
    Loudness         *float64 `json:"loudness,omitempty"`
    Tempo            *float64 `json:"tempo,omitempty"`
    MusicalKey       *int     `json:"musical_key,omitempty"`
    CreatedAt        time.Time `json:"created_at"`
}
```

**Why all feature fields are pointers:**
Every audio feature column is nullable in the database. Pointer types make three distinct states expressible:

| State | Representation |
|-------|---------------|
| Feature not fetched yet | `nil` |
| Feature fetched, value is zero | `Float64Ptr(0.0)` |
| Feature fetched, value is non-zero | `Float64Ptr(0.85)` |

Without pointer types, `0.0` and "not available" are indistinguishable — a critical ambiguity for features like `Instrumentalness` where `0.0` is a meaningful real value (fully vocal track).

---

### Methods on `SongFeature`

**`KeyName() string`**
Converts the integer `MusicalKey` field to a human-readable string using Spotify's Pitch Class notation:

```
0→C, 1→C#, 2→D, 3→D#, 4→E, 5→F, 6→F#, 7→G, 8→G#, 9→A, 10→A#, 11→B
```

Returns `"Unknown"` for nil, -1, or any value outside 0–11.

**`IsComplete() bool`**
Returns `true` only when all ten audio feature fields are non-nil. Used to determine whether a Spotify API fetch is still needed for a track. A track is either fully featured or needs fetching — partial states are treated as incomplete.

---

### Helper Functions (Package-Level)

These are defined in `song_feature.go` and are available to any package that imports `models`:

| Function | Signature | Purpose |
|----------|-----------|---------|
| `Float64Val` | `(*float64) → float64` | Safe dereference, returns `0.0` for nil |
| `IntVal` | `(*int) → int` | Safe dereference, returns `0` for nil |
| `HasFeature` | `(*float64) → bool` | Distinguishes nil from pointer-to-zero |
| `Float64Ptr` | `(float64) → *float64` | Convenience constructor for pointer fields |
| `IntPtr` | `(int) → *int` | Convenience constructor for pointer fields |
| `StringPtr` | `(string) → *string` | Convenience constructor for pointer fields |

**The critical distinction — `Float64Val` vs `HasFeature`:**

```go
Float64Val(nil)             // → 0.0
Float64Val(Float64Ptr(0.0)) // → 0.0  (same return value)

HasFeature(nil)             // → false (not available)
HasFeature(Float64Ptr(0.0)) // → true  (available, value is 0.0)
```

`Float64Val` alone cannot distinguish "not available" from "value is zero". `HasFeature` must be called first when that distinction matters.

---

## JSON Strategy — Applied Consistently Across All Three Structs

| Rule | Implementation |
|------|---------------|
| Field names are `snake_case` | `json:"field_name"` tags on every field |
| Nullable fields absent when nil | `omitempty` on every pointer field |
| Non-nullable fields always present | No `omitempty` on `ID`, `Source`, `FileHash`, `SongID`, `CreatedAt`, `UpdatedAt` |
| Time fields in RFC3339 format | `time.Time` marshals to RFC3339 automatically in Go |

---

## `song_test.go` — What Was Tested

| Test | What It Verifies |
|------|-----------------|
| `TestSong_ZeroValue_NoPanic` | Empty Song calls no method panics |
| `TestNewSong_SourceIsManual` | Constructor sets `Source="manual"` |
| `TestNewSong_FileHashSet` | Constructor sets `FileHash` correctly |
| `TestNewSong_NullableFieldsAreNil` | All nullable fields start as nil |
| `TestSong_SongTitle_NilReturnsEmpty` | Nil Title returns empty string |
| `TestSong_SongTitle_NonNilReturnsValue` | Non-nil Title returns its value |
| `TestSong_SongArtist_NilReturnsEmpty` | Nil Artist returns empty string |
| `TestSong_SongArtist_NonNilReturnsValue` | Non-nil Artist returns its value |
| `TestSong_SongAlbum_NilReturnsEmpty` | Nil Album returns empty string |
| `TestSong_SongAlbum_NonNilReturnsValue` | Non-nil Album returns its value |
| `TestSong_HasSpotifyID_NilIsFalse` | Nil SpotifyID returns false |
| `TestSong_HasSpotifyID_EmptyStringIsFalse` | Empty string SpotifyID returns false |
| `TestSong_HasSpotifyID_ValidIDIsTrue` | Valid SpotifyID returns true |
| `TestSong_IsManualUpload_ManualIsTrue` | Source="manual" returns true |
| `TestSong_IsManualUpload_SpotifyIsFalse` | Source="spotify" returns false |
| `TestSong_DisplayName_BothKnown` | "Artist - Title" format when both known |
| `TestSong_DisplayName_TitleOnly` | Title alone when no artist |
| `TestSong_DisplayName_ArtistOnly` | Artist alone when no title |
| `TestSong_DisplayName_NeitherKnown` | "Unknown Track" fallback |
| `TestIsValidSource_ValidValues` | manual, spotify, unknown all pass |
| `TestIsValidSource_InvalidValues` | youtube, empty, mixed case all fail |
| `TestSong_JSON_FieldNames` | snake_case keys present in JSON |
| `TestSong_JSON_NilFieldsOmitted` | Nil fields absent from JSON output |
| `TestSong_JSON_NonNilFieldsPresent` | Non-nil fields present in JSON output |
| `TestSong_JSON_RoundTrip` | Marshal then unmarshal recovers original values |

---

## `fingerprint_test.go` — What Was Tested

| Test | What It Verifies |
|------|-----------------|
| `TestFingerprint_ZeroValue_NoPanic` | Empty Fingerprint accessible without panic |
| `TestNewFingerprint_FieldsSet` | Constructor sets Hash, SongID, AnchorTime |
| `TestNewFingerprint_IDIsZero` | ID is zero before INSERT |
| `TestFingerprint_Hash_MaxInt32` | Hash field holds MaxInt32 correctly |
| `TestFingerprint_Hash_MinInt32` | Hash field holds MinInt32 correctly |
| `TestFingerprint_Hash_Zero` | Hash field holds zero correctly |
| `TestFingerprint_ID_LargeValue` | int64 ID holds values above int32 max |
| `TestHashResult_ZeroValue_NoPanic` | Empty HashResult accessible without panic |
| `TestHashResult_Fields` | HashResult fields set and read correctly |
| `TestFingerprint_JSON_FieldNames` | snake_case keys present in JSON |
| `TestFingerprint_JSON_RoundTrip` | Marshal then unmarshal recovers original values |

---

## `song_feature_test.go` — What Was Tested

| Test | What It Verifies |
|------|-----------------|
| `TestSongFeature_ZeroValue_NoPanic` | Empty SongFeature methods called without panic |
| `TestNewSongFeature_SongIDSet` | Constructor sets SongID correctly |
| `TestNewSongFeature_AllFeaturesNil` | All ten feature fields start nil |
| `TestSongFeature_IsComplete_AllNil` | Returns false when all features nil |
| `TestSongFeature_IsComplete_SomeNil` | Returns false when any feature nil |
| `TestSongFeature_IsComplete_AllSet` | Returns true when all features populated |
| `TestSongFeature_KeyName_NilReturnsUnknown` | Nil key returns "Unknown" |
| `TestSongFeature_KeyName_NegativeReturnsUnknown` | Key=-1 returns "Unknown" |
| `TestSongFeature_KeyName_AllTwelveKeys` | All 12 pitch classes return correct names |
| `TestSongFeature_KeyName_OutOfRangeReturnsUnknown` | Key=12 returns "Unknown" |
| `TestFloat64Val_NilReturnsZero` | Nil pointer returns 0.0 |
| `TestFloat64Val_NonNilReturnsValue` | Non-nil pointer returns its value |
| `TestFloat64Val_ZeroValueDistinguishable` | Both nil and zero-ptr return 0.0 |
| `TestIntVal_NilReturnsZero` | Nil pointer returns 0 |
| `TestIntVal_NonNilReturnsValue` | Non-nil pointer returns its value |
| `TestHasFeature_NilIsFalse` | Nil returns false |
| `TestHasFeature_NonNilIsTrue` | Non-nil pointer to 0.0 returns true |
| `TestStringPtr_ReturnsPointer` | Returns non-nil pointer to correct value |
| `TestFloat64Ptr_ReturnsPointer` | Returns non-nil pointer to correct value |
| `TestIntPtr_ReturnsPointer` | Returns non-nil pointer to correct value |
| `TestSongFeature_JSON_NilFeaturesOmitted` | All nil features absent from JSON |
| `TestSongFeature_JSON_PopulatedFeaturesPresent` | Populated features present in JSON |
| `TestSongFeature_JSON_FieldNames` | Required keys present in JSON output |

---

## Boundaries Established

The models package establishes the data contract for the entire application:

```
BEFORE models/                    AFTER models/
──────────────                    ─────────────
Raw database rows           →     Typed Go structs
Raw JSON bodies             →     Typed Go structs
Ambiguous nullable values   →     Explicit *pointer types
Magic sentinel values       →     nil means absent
```

---

## What `models/` Deliberately Does Not Contain

| Concern | Handled By |
|---------|-----------|
| SQL queries | `store/song_store.go` |
| HTTP serialization | `api/dto/` |
| Input validation | `api/handlers/` |
| Business rules | `store/` or `matcher/` |
| Default timestamps | `store/` (set by PostgreSQL) |
| Spotify API calls | Phase 7 |

---

## Test Results Achieved

```
All tests  → PASS (song, fingerprint, song_feature)
go vet     → clean
go build   → clean
go clean -testcache confirmed cache cleared before full run
```

---

## What Comes Next — Step 2: `store/`

With models complete, the store layer has the types it needs to read and write to PostgreSQL.

```
Next files to build:

  internal/store/song_store.go
      Insert, GetByID, GetByFileHash, List, Delete

  internal/store/fingerprint_store.go
      InsertBatch (bulk INSERT — performance critical)
      QueryByHashes (SELECT with hash index — matching critical)
```

> This summary reflects the state of `models/song.go`, `models/fingerprint.go`, and `models/song_feature.go` as committed. Move forward to `store docs` for further development explanation.