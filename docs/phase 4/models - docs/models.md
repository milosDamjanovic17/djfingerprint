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