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