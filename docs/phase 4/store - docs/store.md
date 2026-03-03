# Phase 4 — Step 2 — `store/` Overview

---

## What the Store Package Is

The `store` package is the **only layer in the entire application that speaks SQL**. Nothing outside this package writes a query, opens a transaction, or touches a database connection directly. Every other layer that needs data goes through the store.

```
api/handlers/    →  calls store methods
matcher/         →  receives results from store
models/          →  structs that store reads/writes
store/           →  the only SQL in the application
PostgreSQL       →  the actual database
```

---

## Why a Dedicated Store Package

Without a store layer, SQL queries end up scattered across handlers, business logic, and helper functions. This creates several problems:

- The same query gets written in multiple places
- Testing requires a real HTTP server to test database behaviour
- Changing a table schema means hunting through the entire codebase
- Database errors surface in unexpected places with no consistent handling

The store package solves all of this by being the **single point of contact** with PostgreSQL.

---

## The Two Files

### `store/song_store.go`

Responsible for all reads and writes to the `songs` table. Five operations:

```
Insert(song *models.Song) (*models.Song, error)
    → INSERT a new song, returns the song with ID and timestamps populated

GetByID(id int) (*models.Song, error)
    → SELECT one song by primary key
    → Returns a typed error if not found

GetByFileHash(hash string) (*models.Song, error)
    → SELECT one song by file_hash
    → Used during upload to detect duplicates before fingerprinting

List(page, pageSize int) ([]*models.Song, int, error)
    → SELECT paginated list of songs
    → Returns songs, total count, error

Delete(id int) error
    → DELETE one song by primary key
    → Fingerprints removed automatically via CASCADE
```

---

### `store/fingerprint_store.go`

Responsible for all reads and writes to the `fingerprints` table. Two operations — both performance critical:

```
InsertBatch(hashes []dsp.FingerprintHash) error
    → Bulk INSERT all hashes for a song in one operation
    → A single song generates ~58,000 rows
    → Must use batch insertion, not one INSERT per row

QueryByHashes(hashes []dsp.FingerprintHash) ([]models.HashResult, error)
    → SELECT all fingerprint rows matching any of the query hashes
    → This is the matching query — runs on every identification request
    → Uses idx_fingerprints_hash, the critical index from Phase 1
```

---

## The Database Connection Design

Neither store manages its own database connection. Both receive a `*sql.DB` from the outside — this is called **dependency injection**. The connection is opened once in `main.go` and passed down.

```go
// Both stores follow this pattern
type SongStore struct {
    db *sql.DB
}

func NewSongStore(db *sql.DB) *SongStore {
    return &SongStore{db: db}
}
```

**Why dependency injection:**

| Concern | Result |
|---------|--------|
| Testing | Pass a test database connection without changing store code |
| Connection pooling | One `*sql.DB` pool shared across all stores |
| Startup control | `main.go` controls when the connection opens and closes |
| Multiple databases | Could point stores at different DBs if ever needed |

---

## The Batch Insert Strategy

A single song generates approximately 58,000 fingerprint rows. Inserting them one by one with individual `INSERT` statements would require 58,000 round trips to PostgreSQL. On a local connection this would take several seconds. This is unacceptable.

Instead we use **batched inserts** — grouping rows into chunks and sending multiple values per `INSERT` statement:

```sql
-- Instead of 58,000 of these:
INSERT INTO fingerprints (hash, song_id, anchor_time) VALUES (?, ?, ?);

-- We send ~580 of these (100 rows per batch):
INSERT INTO fingerprints (hash, song_id, anchor_time)
VALUES (?, ?, ?), (?, ?, ?), (?, ?, ?), ... (100 rows per statement)
```

**Batch size of 100** rows per statement is the chosen balance:

| Batch Size | Round Trips (58k rows) | Risk |
|-----------|----------------------|------|
| 1 | 58,000 | Too slow |
| 100 | 580 | Good balance |
| 1000 | 58 | Approaches PostgreSQL parameter limits |
| 65535 | 1 | Exceeds PostgreSQL max parameters (65535) |

PostgreSQL has a hard limit of **65,535 parameters per statement**. With 3 columns per row, the maximum rows per statement is $\lfloor 65535 / 3 \rfloor = 21845$. A batch size of 100 stays well within this limit.

---

## The Matching Query Design

The identification query needs to find all fingerprint rows that match any hash from the query snippet. The snippet generates approximately 500–2000 hashes for a 10–15 second clip.

```sql
SELECT song_id, anchor_time, hash
FROM fingerprints
WHERE hash IN (?, ?, ?, ... up to 2000 values)
```

This query hits `idx_fingerprints_hash` directly — the index established in Phase 1. PostgreSQL resolves this with an index scan rather than a sequential table scan regardless of how many rows are in the table.

**Why `IN` rather than multiple `SELECT` statements:**
One query with an `IN` clause is significantly faster than issuing one `SELECT` per hash value. The query planner handles the `IN` list efficiently against the B-tree index.

---

## Testing Strategy

The store tests require a real database — you cannot meaningfully test SQL behaviour without executing SQL. We use **two levels**:

### Unit Tests — SQLite in-memory

```go
// Fast, no PostgreSQL required, runs anywhere
db, _ := sql.Open("sqlite3", ":memory:")
store := NewSongStore(db)
```

Used for:
- CRUD correctness
- Duplicate detection
- Pagination logic
- Cascade delete verification

### Integration Tests — Real PostgreSQL

```go
// Uses your local djfingerprint database
// Runs against a dedicated test schema
// Cleaned up after each test
db, _ := sql.Open("postgres", cfg.DSNPostgres())
```

Used for:
- Batch insert performance verification
- Index usage confirmation via `EXPLAIN ANALYZE`
- Hash query correctness against real data volumes

---

## Additional Dependency Required

The store package needs two drivers:

```powershell
# PostgreSQL driver (already installed in Phase 2 setup)
# Confirm it is present
go list -m github.com/lib/pq

# SQLite driver for tests
go get github.com/mattn/go-sqlite3
```

> ⚠️ `go-sqlite3` uses CGO. On Windows this requires a C compiler.
> The easiest way to get one is **TDM-GCC**:
> https://jmeubank.github.io/tdm-gcc/
> Download, install, restart your terminal, then run `go get github.com/mattn/go-sqlite3`

If you want to avoid CGO entirely, we can use `modernc.org/sqlite` which is a pure Go SQLite implementation:

```powershell
go get modernc.org/sqlite
```

We will confirm which one you have available before writing the code.

---

## What the Store Package Does NOT Contain

| Concern | Handled By |
|---------|-----------|
| Business logic | `api/handlers/` |
| Hash generation | `dsp/fingerprint.go` |
| Coherence scoring | `matcher/matcher.go` |
| HTTP concerns | `api/handlers/` |
| Connection string building | `internal/config/config.go` |
| Migration execution | `db/migrations/` |

---

## Store Package Completion Criteria

```
[ ] SongStore.Insert correctly persists a song and returns ID
[ ] SongStore.GetByID returns correct song or typed not-found error
[ ] SongStore.GetByFileHash detects duplicate uploads correctly
[ ] SongStore.List returns correct page and total count
[ ] SongStore.Delete removes song (fingerprints cascade verified)
[ ] FingerprintStore.InsertBatch inserts all rows correctly
[ ] FingerprintStore.InsertBatch handles batch boundary correctly
    (e.g. 101 rows → 2 batches: 100 + 1)
[ ] FingerprintStore.QueryByHashes returns correct matches
[ ] FingerprintStore.QueryByHashes returns empty slice for no matches
[ ] All tests pass against SQLite (unit) and PostgreSQL (integration)
[ ] go vet clean
[ ] go build clean
```

---

## Build Order Within store/

```
1. song_store.go first
   Simpler operations, establishes the pattern
   Tests verify basic DB interaction works

2. fingerprint_store.go second
   Depends on the batch insert pattern being established
   Performance tests require songs to exist (song_store needed first)
```

---

# `song_store.go` & `song_store_test.go` — Build Summary

---

## What We Built

### The Purpose
`song_store.go` is the **sole point of contact between the application and the `songs` table in PostgreSQL**. Nothing outside this file writes a SQL query that touches the songs table. Every other layer that needs song data — handlers, matcher, dto — goes through the `SongStore` methods.

---

## `song_store.go` — What Was Built and Why

### The `SongStore` Struct

```go
type SongStore struct {
    db *sql.DB
}

func NewSongStore(db *sql.DB) *SongStore {
    return &SongStore{db: db}
}
```

The store holds a reference to a `*sql.DB` connection pool — it does not own or manage the connection. The pool is opened once in `main.go` and injected here via the constructor. This is **dependency injection** — the store does not care where the connection came from, only that it works.

**Why dependency injection matters for testing:**
The same `SongStore` code runs against the real `djfingerprint` database in production and against the test database in tests. No conditional logic, no build tags, no mocking — just pass a different `*sql.DB`.

---

### Sentinel Errors

```go
var ErrNotFound      = errors.New("record not found")
var ErrDuplicateHash = errors.New("song with this file hash already exists")
```

Two package-level errors are defined rather than returning raw database errors to callers:

| Error | When Returned | HTTP Meaning |
|-------|--------------|--------------|
| `ErrNotFound` | `GetByID`, `GetByFileHash`, `Delete` when no row matches | 404 Not Found |
| `ErrDuplicateHash` | `Insert` when `file_hash` unique constraint fires | 409 Conflict |

Callers use `errors.Is()` to check for these specifically:

```go
if errors.Is(err, store.ErrNotFound) {
    // return HTTP 404
}
if errors.Is(err, store.ErrDuplicateHash) {
    // return HTTP 409
}
```

This separation means the handler layer never needs to parse PostgreSQL error codes or error strings — it only deals with typed Go errors.

---

### `Insert(song *models.Song) (*models.Song, error)`

Persists a new song row and returns the song with PostgreSQL-assigned fields populated.

**Key implementation details:**

**`RETURNING` clause:**
```sql
INSERT INTO songs (...) VALUES (...)
RETURNING id, created_at, updated_at
```
PostgreSQL assigns `id` (SERIAL), `created_at` and `updated_at` (DEFAULT NOW()) at insert time. The `RETURNING` clause retrieves these in the same round trip — no second `SELECT` query needed.

**Nullable field conversion:**
Go's `*string` and `*int` pointer types cannot be passed directly to `sql.QueryRow`. They must be converted to `sql.NullString` and `sql.NullInt64`:

```go
nullableString(song.Title)    // *string → sql.NullString
nullableInt(song.DurationMs)  // *int    → sql.NullInt64
```

`nil` pointer becomes `sql.NullString{Valid: false}` which PostgreSQL receives as `NULL`.

**Duplicate detection:**
PostgreSQL fires error code `23505` (unique_violation) when `file_hash` conflicts. The `isUniqueViolation` helper checks the error string for this code and returns `ErrDuplicateHash` — a clean typed error the handler can respond to with HTTP 409.

---

### `GetByID(id int) (*models.Song, error)`

Retrieves a single song by primary key.

**`sql.ErrNoRows` translation:**
When no row matches, `sql.QueryRow().Scan()` returns `sql.ErrNoRows`. This is translated to `ErrNotFound` before returning to the caller:

```go
if errors.Is(err, sql.ErrNoRows) {
    return nil, ErrNotFound
}
```

The caller never sees `sql.ErrNoRows` — only our application-level `ErrNotFound`.

---

### `GetByFileHash(hash string) (*models.Song, error)`

Retrieves a single song by its SHA256 file hash. Called during upload before fingerprinting to detect duplicate files early — before the expensive DSP pipeline runs.

**Why check before fingerprinting:**
Fingerprinting a 3-minute song takes several seconds and generates ~58,000 database rows. Detecting a duplicate in milliseconds via a single indexed query avoids all of that work.

---

### `List(page, pageSize int) ([]*models.Song, int, error)`

Returns a paginated list of songs ordered by `created_at DESC` (newest first).

**Pagination design:**
- `page` is 1-indexed — page 1 returns the first batch
- `pageSize` defaults to 20 if below 1, capped at 100 to prevent large result sets
- Returns three values: the songs slice, the total count, and any error
- Total count is fetched first in a separate `COUNT(*)` query so the frontend can calculate total pages

**Why total count matters:**
```
totalPages = ceil(total / pageSize)
```
Without total count the frontend cannot know whether there are more pages to fetch.

**Empty database shortcut:**
If `COUNT(*)` returns 0, the list query is skipped entirely and an empty slice is returned immediately.

---

### `Delete(id int) error`

Removes a song by primary key. Returns `ErrNotFound` when no row is affected.

**CASCADE delete:**
The `ON DELETE CASCADE` constraint defined in Migration 002 means deleting a song automatically removes all associated rows from the `fingerprints` table. The store does not need to issue a separate `DELETE FROM fingerprints` statement — PostgreSQL handles it atomically.

**`RowsAffected()` check:**
```go
rowsAffected, _ := result.RowsAffected()
if rowsAffected == 0 {
    return ErrNotFound
}
```
`db.Exec` does not return an error when a `DELETE` matches zero rows — it simply affects zero rows. The explicit `RowsAffected()` check catches the case where the caller passes an ID that does not exist.

---

### Private Helper Functions

| Helper | Purpose |
|--------|---------|
| `scanSong(*sql.Row)` | Scans a single `QueryRow` result into `*models.Song` |
| `scanSongRow(*sql.Rows)` | Scans a multi-row `Query` result into `*models.Song` |
| `nullableString(*string)` | Converts Go `*string` → `sql.NullString` for query params |
| `nullableInt(*int)` | Converts Go `*int` → `sql.NullInt64` for query params |
| `toStringPtr(sql.NullString)` | Converts `sql.NullString` scan result → Go `*string` |
| `toIntPtr(sql.NullInt64)` | Converts `sql.NullInt64` scan result → Go `*int` |
| `isUniqueViolation(error)` | Checks whether a PostgreSQL error is code 23505 |
| `contains(string, string)` | Substring check used by `isUniqueViolation` |
| `findSubstring(string, string)` | Inner loop of `contains` |

The scan helpers exist because Go's `database/sql` package cannot scan directly into pointer types — it requires `sql.Null*` intermediates. Both `scanSong` and `scanSongRow` contain identical column scan logic but operate on different types (`*sql.Row` vs `*sql.Rows`) which cannot be unified without an interface.

---

## The Schema Fix We Encountered

### What Happened

```
expected FileHash="aabbccdd008", got "aabbccdd008                          "
```

### Root Cause

The `file_hash` column was defined as `CHAR(64)` in Phase 1:

```sql
file_hash CHAR(64) NOT NULL
```

`CHAR(n)` is a **fixed-length** type. PostgreSQL always stores exactly 64 characters by padding shorter values with trailing spaces. `"aabbccdd008"` (11 chars) was stored as `"aabbccdd008"` followed by 53 spaces.

### The Fix

Changed the column type to `VARCHAR(64)`:

```sql
-- In pgAdmin
ALTER TABLE songs ALTER COLUMN file_hash TYPE VARCHAR(64);

-- In db/migrations/001_create_songs.sql
file_hash VARCHAR(64) NOT NULL
```

`VARCHAR(n)` stores exactly what is inserted with no padding. The 64-character maximum is preserved to document that SHA256 hashes are always 64 hex characters.

### The Migration Files

This issue also revealed that all three migration files in `db/migrations/` were empty. They were populated in this session with the full Phase 1 SQL including the `VARCHAR(64)` fix. The migration files are now the source of truth for the schema and are committed to Git.

---

## Testing Strategy Used

### PostgreSQL-Only Decision

SQLite was evaluated and rejected for store tests for these reasons:

- PostgreSQL `CHAR(64)` vs `VARCHAR(64)` behaviour — this exact bug — would not have been caught by SQLite
- `RETURNING` clause syntax differs between PostgreSQL and SQLite
- `$1 $2 $3` parameter placeholders differ from SQLite's `?`
- `ON DELETE CASCADE` enforcement timing differs
- The `djfingerprint` database was already running and configured from Phase 1

### Transaction Rollback Isolation

Each test opens its own `*sql.DB` connection and uses explicit cleanup via `defer db.Exec("DELETE FROM songs WHERE id = $1", id)` rather than transaction rollback. This ensures:
- No test data persists in `djfingerprint` after the test run
- Tests are independent of each other
- The real PostgreSQL constraints are exercised

### `DB_PASSWORD` Environment Variable

Tests read database credentials from environment variables:

```powershell
$env:DB_PASSWORD = "your_password"
```

If `DB_PASSWORD` is not set, tests call `t.Skip()` rather than `t.Fatal()`. This means the test suite degrades gracefully on machines without PostgreSQL rather than failing noisily.

---

## `song_store_test.go` — What Was Tested

### Test Helper File — `store_test_helpers_test.go`

Three shared helpers established for all store tests:

**`testDB(t *testing.T) *sql.DB`**
Opens and pings a real PostgreSQL connection. Reads credentials from environment variables. Calls `t.Skip()` if the database is unreachable — tests are skipped, not failed.

**`testSong(fileHash string) *models.Song`**
Creates a minimal valid `Song` with only `Source` and `FileHash` set. All nullable fields left nil.

**`testSongWithMeta(fileHash string) *models.Song`**
Creates a `Song` with `Title`, `Artist`, `Album`, `Source`, and `FileHash` all populated. Used for tests that verify metadata round-trips correctly.

---

### Tests Written

| Test | What It Verifies |
|------|-----------------|
| `TestSongStore_Insert_Success` | Song inserted, ID non-zero, timestamps populated |
| `TestSongStore_Insert_WithMetadata` | Title, Artist, Album persisted and retrieved correctly |
| `TestSongStore_Insert_NullableFieldsNil` | Nil fields remain nil after insert and retrieve |
| `TestSongStore_Insert_DuplicateHash_ReturnsDuplicateError` | Second insert with same hash returns `ErrDuplicateHash` |
| `TestSongStore_Insert_SourceDefaultManual` | Source field persisted correctly |
| `TestSongStore_GetByID_Success` | Correct song returned by primary key |
| `TestSongStore_GetByID_NotFound` | `ErrNotFound` returned for nonexistent ID |
| `TestSongStore_GetByID_NullableFieldsRoundTrip` | Nil fields scan back as nil not empty string |
| `TestSongStore_GetByFileHash_Success` | Correct song returned by file hash |
| `TestSongStore_GetByFileHash_NotFound` | `ErrNotFound` returned for nonexistent hash |
| `TestSongStore_List_Empty` | List does not error on empty or low-count database |
| `TestSongStore_List_ReturnsInsertedSongs` | Inserted songs appear in list results |
| `TestSongStore_List_Pagination` | Page 1 and Page 2 contain different song IDs |
| `TestSongStore_List_PageSizeCappedAt100` | Requesting 9999 returns at most 100 |
| `TestSongStore_List_InvalidPageDefaults` | Page 0 and negative page do not error |
| `TestSongStore_Delete_Success` | Song deleted, subsequent GetByID returns ErrNotFound |
| `TestSongStore_Delete_NotFound` | `ErrNotFound` returned for nonexistent ID |
| `TestSongStore_Delete_RemovesOnlyTargetSong` | Only targeted song deleted, sibling song intact |

---

## Boundaries Established

`song_store.go` owns everything related to the `songs` table:

```
BEFORE song_store.go               AFTER song_store.go
────────────────────               ───────────────────
Raw SQL queries              →     Typed Go method calls
sql.NullString intermediates →     *string pointer types
PostgreSQL error codes       →     ErrNotFound / ErrDuplicateHash
Three separate queries       →     One Insert call with RETURNING
```

---

## What `song_store.go` Deliberately Does Not Do

| Concern | Handled By |
|---------|-----------|
| Fingerprint INSERT/SELECT | `store/fingerprint_store.go` — next file |
| File hash calculation | `api/handlers/upload.go` |
| Duplicate detection logic | `api/handlers/upload.go` calls `GetByFileHash` |
| Audio processing | `audio/` and `dsp/` packages |
| HTTP responses | `api/handlers/` |
| Connection management | `cmd/api/main.go` |

---

## Test Results Achieved

```
All tests  → PASS
No data    → left in djfingerprint database after test run
go vet     → clean
go build   → clean
```

---

## What Comes Next — `fingerprint_store.go`

With song CRUD complete, the fingerprint store builds on the same patterns:

```
fingerprint_store.go
  └── InsertBatch(hashes []dsp.FingerprintHash) error
        Bulk INSERT ~58,000 rows per song in batches of 100

  └── QueryByHashes(hashes []dsp.FingerprintHash) ([]models.HashResult, error)
        SELECT matching rows using idx_fingerprints_hash
        The single most performance-critical query in the application
```

> This summary reflects the state of `song_store.go` and `song_store_test.go` as committed. Move forward to `fingerprint_store.go` to check the fingerprinting process.