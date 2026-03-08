# `fingerprint_store.go` — Introduction Document

---

## What `fingerprint_store.go` Does

```
Receives  → []dsp.FingerprintHash from the DSP engine
Does      → bulk INSERT into the fingerprints table (indexing path)
            SELECT matching rows by hash value (query path)
Returns   → []models.HashResult for the matcher to score
```

## What `fingerprint_store.go` Does NOT Do

```
Does not generate hashes          ← dsp/fingerprint.go
Does not score or rank results    ← matcher/matcher.go
Does not touch the songs table    ← song_store.go
Does not know about songs         ← only hash integers and song IDs
Does not manage connections       ← main.go
```

---

## Two Operations — Both Performance Critical

Every other store operation in this application is a simple CRUD call. The fingerprint store is different — both of its operations deal with tens of thousands of rows and sit directly on the hot path of the two most important user actions:

```
Upload a song    → InsertBatch   → ~58,000 INSERT rows
Identify audio   → QueryByHashes → SELECT against millions of rows
```

Performance is not a nice-to-have here — it is a hard requirement.

---

## Operation 1 — `InsertBatch`

### The Problem With Naive Insertion

A single song generates approximately 58,000 fingerprint hashes. The naive approach:

```go
for _, hash := range hashes {
    db.Exec("INSERT INTO fingerprints VALUES ($1, $2, $3)", ...)
}
```

This sends **58,000 individual round trips** to PostgreSQL. On a localhost connection this takes 10–30 seconds. Completely unacceptable for a user uploading a track.

### The Solution — Batched Multi-Row INSERT

Group rows into chunks and send multiple values per statement:

```sql
INSERT INTO fingerprints (hash, song_id, anchor_time)
VALUES
    ($1,  $2,  $3),
    ($4,  $5,  $6),
    ($7,  $8,  $9),
    ...
    ($298, $299, $300)   -- 100 rows = 300 parameters per statement
```

With a batch size of 100:

$$\text{round trips} = \lceil 58{,}000 / 100 \rceil = 580$$

580 round trips instead of 58,000. Approximately **100x fewer network calls**.

### Why Batch Size 100

PostgreSQL has a hard limit of **65,535 parameters per statement**. Each fingerprint row uses 3 parameters. Maximum safe rows per statement:

$$\lfloor 65535 / 3 \rfloor = 21{,}845$$

We use 100 — well within the limit, large enough to be efficient, small enough to keep statement construction fast.

| Batch Size | Round Trips | Parameters Per Statement | Risk |
|-----------|-------------|------------------------|------|
| 1 | 58,000 | 3 | Too slow |
| 100 | 580 | 300 | ✅ Optimal |
| 1000 | 58 | 3,000 | Fine but diminishing returns |
| 21845 | 3 | 65,535 | At PostgreSQL limit |

### Handling the Last Batch

58,000 rows divides into 580 batches of exactly 100. But for 58,001 rows:

```
Batch 1:   rows 1–100    (full batch of 100)
Batch 2:   rows 101–200  (full batch of 100)
...
Batch 580: rows 57901–58000 (full batch of 100)
Batch 581: rows 58001–58001 (partial batch of 1)
```

The last batch must be handled without error regardless of its size. The batch loop must use the actual remaining row count, not assume a full batch.

### Dynamic SQL Construction

Because the number of parameters changes per batch, the SQL statement cannot be a fixed string — it must be built dynamically:

```go
// For a batch of 3 rows:
"INSERT INTO fingerprints (hash, song_id, anchor_time) VALUES ($1,$2,$3),($4,$5,$6),($7,$8,$9)"

// For a batch of 1 row:
"INSERT INTO fingerprints (hash, song_id, anchor_time) VALUES ($1,$2,$3)"
```

The parameter placeholders `$1`, `$2`, `$3`... must be numbered sequentially across the entire statement, not reset per row.

---

## Operation 2 — `QueryByHashes`

### What It Does

When a user submits a 15-second audio snippet for identification:

1. The DSP engine generates ~1,000–2,000 hashes from the snippet
2. `QueryByHashes` takes those hashes and finds all matching rows in the `fingerprints` table
3. Returns every matching `(song_id, anchor_time, hash)` triplet
4. The matcher receives these results and performs coherence scoring

### The SQL

```sql
SELECT song_id, anchor_time, hash
FROM fingerprints
WHERE hash = ANY($1)
```

**Why `ANY($1)` instead of `IN (...)`:**

PostgreSQL supports passing an array directly as a single parameter using `ANY`:

```go
// Clean — one parameter, one array
db.Query("SELECT ... WHERE hash = ANY($1)", pq.Array(hashValues))
```

vs building a dynamic `IN` clause:

```go
// Messy — requires building ($1,$2,$3,...,$2000) dynamically
"WHERE hash IN ($1,$2,$3,...,$2000)"
```

`pq.Array()` from the `github.com/lib/pq` driver converts a Go `[]int32` slice into a PostgreSQL array that `ANY` can use. One parameter regardless of how many hash values are in the query.

### Index Usage

This query hits `idx_fingerprints_hash` directly — the B-tree index created in Phase 1 on the `hash` column. PostgreSQL resolves the array lookup with an index scan rather than a sequential table scan regardless of how many rows are in the table.

For a library of 1000 songs (~58 million rows), this query returns in milliseconds.

### Result Volume

For a 15-second snippet with ~1,500 query hashes, the result set could contain thousands of matching rows from many different songs. The matcher receives all of them and uses coherence scoring to identify which song the matches actually belong to.

---

## The `FingerprintStore` Struct

Follows the exact same pattern as `SongStore`:

```go
type FingerprintStore struct {
    db *sql.DB
}

func NewFingerprintStore(db *sql.DB) *FingerprintStore {
    return &FingerprintStore{db: db}
}
```

---

## Connection to the Rest of the Application

```
Upload path:
  handlers/upload.go
        │
        ▼ GenerateHashes()
  dsp/fingerprint.go → []FingerprintHash
        │
        ▼ InsertBatch()
  fingerprint_store.go → INSERT INTO fingerprints

Identify path:
  handlers/identify.go
        │
        ▼ GenerateHashes()
  dsp/fingerprint.go → []FingerprintHash
        │
        ▼ QueryByHashes()
  fingerprint_store.go → []models.HashResult
        │
        ▼ Match()
  matcher/matcher.go → Top 3 results
```

---

## Testing Strategy

### InsertBatch Tests
- Insert a known number of hashes and verify row count in the database
- Verify batch boundary handling — 101 rows uses 2 batches (100 + 1)
- Verify all rows belong to the correct song ID
- Verify empty input is handled without error

### QueryByHashes Tests
- Insert known hashes for a song, query for them, verify results returned
- Query for hashes that do not exist — verify empty result, no error
- Query after inserting multiple songs — verify only matching song IDs returned
- Verify `song_id` and `anchor_time` values are correct in results

### Performance Test
- Insert 58,000 rows (one full song worth)
- Measure total insert time — must be under 10 seconds
- Query 1,500 hashes against the table — must return under 500ms

---

## What the Tests Will Require

Each test that inserts fingerprints needs a valid `song_id` because of the foreign key constraint on the `fingerprints` table. The test setup will:

1. Insert a throwaway song using `SongStore.Insert`
2. Use that song's ID for fingerprint rows
3. Delete the song at the end — cascade removes all fingerprint rows automatically

This means `fingerprint_store_test.go` depends on `SongStore` being functional — which it is, having been built and tested already.

---

## Phase 1 Index Reminder

The entire performance model of this application rests on this index existing:

```sql
CREATE INDEX idx_fingerprints_hash ON fingerprints(hash);
```

If this index is ever dropped or becomes invalid, `QueryByHashes` degrades from milliseconds to minutes on a large library. Every test that queries by hash will verify the index is being used via `EXPLAIN` output in the performance test.

---

> Introduction document part complete. Below is the summary of what fingerpint_store.go does

# `fingerprint_store.go` & `fingerprint_store_test.go` — Build Summary

---

## What We Built

### The Purpose
`fingerprint_store.go` is the **performance-critical layer between the DSP engine and PostgreSQL**. It has exactly two operations — one for writing fingerprints during song indexing and one for reading them during identification. Both operations deal with tens of thousands of rows and sit directly on the hot path of the two most important user actions in the entire application.

---

## `fingerprint_store.go` — What Was Built and Why

### The `FingerprintStore` Struct

```go
type FingerprintStore struct {
    db *sql.DB
}

func NewFingerprintStore(db *sql.DB) *FingerprintStore {
    return &FingerprintStore{db: db}
}
```

Identical pattern to `SongStore` — receives a `*sql.DB` connection pool via dependency injection. Does not open, close, or manage the connection. The pool is shared across all stores and managed by `main.go`.

---

### The Batch Size Constant

```go
const insertBatchSize = 100
```

The single most important constant in this file. Governs how many rows are grouped per INSERT statement.

**The calculation that justifies it:**

PostgreSQL hard limit: $65{,}535$ parameters per statement

Each fingerprint row uses 3 parameters (`hash`, `song_id`, `anchor_time`):

$$\text{max rows per statement} = \left\lfloor \frac{65535}{3} \right\rfloor = 21{,}845$$

We use 100 — well within the limit and proven to reduce 58,000 individual round trips down to 580 batched statements:

$$\text{round trips} = \left\lceil \frac{58{,}000}{100} \right\rceil = 580$$

---

### `InsertBatch(hashes []dsp.FingerprintHash) error`

Bulk inserts all fingerprint hashes for a song using batched multi-row INSERT statements wrapped in a single transaction.

**What it does step by step:**

```
1. Guard: empty slice is a no-op, return nil immediately
        ↓
2. Open a database transaction
        ↓
3. defer tx.Rollback()
   If Commit() succeeds later, Rollback() becomes a no-op
   If anything fails, Rollback() fires automatically
        ↓
4. Slice the hashes into chunks of insertBatchSize:
   for i := 0; i < len(hashes); i += insertBatchSize
        ↓
5. For each chunk call insertBatch(tx, batch)
        ↓
6. tx.Commit()
        ↓
7. Return nil on success
```

**Why a single transaction for all batches:**
A partial fingerprint set is worse than no fingerprints at all. If batch 3 of 580 fails, batches 1 and 2 must be rolled back — leaving 200 orphaned rows with no corresponding complete fingerprint set would cause the matcher to return incorrect low-confidence results. The transaction guarantees all-or-nothing insertion.

---

### `insertBatch(tx *sql.Tx, batch []dsp.FingerprintHash) error`

Private function. Builds and executes a single multi-row INSERT for one batch.

**Dynamic SQL construction:**

The SQL cannot be a fixed string because the number of rows — and therefore the number of `$n` placeholders — varies per batch (the last batch is almost always smaller than `insertBatchSize`).

For a batch of 3 rows:
```sql
INSERT INTO fingerprints (hash, song_id, anchor_time)
VALUES ($1, $2, $3), ($4, $5, $6), ($7, $8, $9)
```

For the last partial batch of 1 row:
```sql
INSERT INTO fingerprints (hash, song_id, anchor_time)
VALUES ($1, $2, $3)
```

**Parameter numbering:**
Parameters are numbered sequentially across the entire statement — `$1`, `$2`, `$3`... continuing without reset between rows. This is required by the `lib/pq` driver.

```go
base := i * 3
valueStrings = append(valueStrings,
    fmt.Sprintf("($%d, $%d, $%d)", base+1, base+2, base+3),
)
```

**Pre-allocation:**
Both `valueStrings` and `valueArgs` are allocated with known capacity before the loop — `make([]string, 0, len(batch))` and `make([]interface{}, 0, len(batch)*3)`. This avoids repeated slice growth during the inner loop.

---

### `QueryByHashes(hashes []dsp.FingerprintHash) ([]models.HashResult, error)`

Finds all fingerprint rows matching any hash in the query slice. The matching query — runs on every identification request.

**The `ANY($1)` approach:**

```sql
SELECT song_id, anchor_time, hash
FROM fingerprints
WHERE hash = ANY($1)
```

```go
hashValues := make([]int32, len(hashes))
for i, h := range hashes {
    hashValues[i] = h.Hash
}
rows, err := fs.db.Query(query, pq.Array(hashValues))
```

`pq.Array()` from the `github.com/lib/pq` driver converts `[]int32` into a PostgreSQL array. The `ANY` operator then tests each row's `hash` column against the array using `idx_fingerprints_hash`.

**Why `ANY($1)` over a dynamic `IN` clause:**

| Approach | Parameters | Complexity |
|----------|-----------|-----------|
| `WHERE hash = ANY($1)` | 1 | Fixed SQL, one array parameter |
| `WHERE hash IN ($1,$2,...,$2000)` | Up to 2000 | Dynamic SQL construction required |

One parameter regardless of query size. Fixed SQL string that never needs building. Same index usage.

**Empty result handling:**
Returns an empty `[]models.HashResult{}` slice when no matches are found — not an error. "No match found" is a valid identification outcome and the matcher handles it gracefully.

---

### `CountBySongID(songID int) (int, error)`

A utility method that counts fingerprint rows for a given song. Not used by the application — exists purely to support test assertions. Allows tests to verify that `InsertBatch` stored exactly the expected number of rows without reading all rows back.

---

## Performance Results Achieved

The performance tests confirmed both operations meet their targets:

### InsertBatch — 58,000 rows

$$\text{actual time} = 1.19 \text{ seconds} \quad (\text{limit: 10 seconds})$$

This is approximately **49× faster than the limit** and demonstrates that batching 580 statements instead of 58,000 individual inserts is highly effective. The theoretical speedup from batching alone is:

$$\text{speedup} = \frac{58{,}000 \text{ round trips}}{580 \text{ round trips}} = 100\times$$

### QueryByHashes — 1,500 hashes against 58,000 rows

$$\text{actual time} = 10.47 \text{ ms} \quad (\text{limit: 500 ms})$$

This is approximately **47× faster than the limit** and confirms that `idx_fingerprints_hash` is being used correctly. A sequential table scan of 58,000 rows would take significantly longer than 10ms even on localhost.

---

## `fingerprint_store_test.go` — What Was Tested and Why

### Test Helper Functions

Two helpers established for all fingerprint store tests:

**`makeHashes(songID int, count int) []dsp.FingerprintHash`**
Generates a slice of deterministic `FingerprintHash` values — hash value equals `i+1`, anchor time equals `i*10`. Deterministic values mean tests are repeatable and specific hashes can be predicted by the test assertions.

**`insertTestSong(t *testing.T, store *SongStore, suffix string) int`**
Inserts a throwaway song using `SongStore` and returns its ID. Required because the `fingerprints.song_id` foreign key constraint prevents inserting fingerprints without a parent song. The caller defers `DELETE FROM songs WHERE id = $1` — cascade removes all fingerprints automatically.

---

### Tests Written

| Test | What It Verifies |
|------|-----------------|
| `TestFingerprintStore_InsertBatch_EmptySlice_NoError` | Empty hash slice is a no-op, not an error |
| `TestFingerprintStore_InsertBatch_CorrectRowCount` | 50 hashes produces exactly 50 rows |
| `TestFingerprintStore_InsertBatch_ExactlyOneBatch` | 100 rows (one full batch) inserts correctly |
| `TestFingerprintStore_InsertBatch_BatchBoundary_101Rows` | 101 rows splits into 2 batches (100+1) without error |
| `TestFingerprintStore_InsertBatch_BatchBoundary_199Rows` | 199 rows splits into 2 batches (100+99) without error |
| `TestFingerprintStore_InsertBatch_SingleRow` | Single row inserts correctly |
| `TestFingerprintStore_InsertBatch_SongID_Correct` | Rows for song1 do not appear under song2 |
| `TestFingerprintStore_InsertBatch_TransactionRollback_OnFailure` | FK violation rolls back all rows, none committed |
| `TestFingerprintStore_QueryByHashes_EmptySlice_ReturnsEmpty` | Empty query returns empty slice, not error |
| `TestFingerprintStore_QueryByHashes_NoMatches_ReturnsEmpty` | Non-existent hashes return empty slice |
| `TestFingerprintStore_QueryByHashes_ReturnsMatchingRows` | Known inserted hashes return results |
| `TestFingerprintStore_QueryByHashes_SongID_InResults` | Query for song1 hashes returns only song1 ID |
| `TestFingerprintStore_QueryByHashes_AnchorTime_Correct` | AnchorTime value round-trips correctly |
| `TestFingerprintStore_QueryByHashes_HashValue_InResults` | Hash value itself present in results |
| `TestFingerprintStore_InsertBatch_Performance_FullSong` | 58,000 rows inserted in under 10 seconds |
| `TestFingerprintStore_QueryByHashes_Performance` | 1,500 hash query returns in under 500ms |
| `TestFingerprintStore_CascadeDelete_RemovesFingerprints` | Deleting song removes all fingerprints via CASCADE |

---

## The Batch Boundary — Key Test Cases

Three specific batch sizes were tested to cover all boundary conditions:

```
50 rows   → 1 partial batch   (below insertBatchSize)
100 rows  → 1 full batch      (exactly insertBatchSize)
101 rows  → 2 batches: 100+1  (one full + one single-row partial)
199 rows  → 2 batches: 100+99 (one full + one large partial)
```

The boundary at `insertBatchSize + 1` (101 rows) is the most important — it is where the loop transitions from one batch to two and where off-by-one errors are most likely to appear.

---

## Boundaries Established

`fingerprint_store.go` owns everything related to the `fingerprints` table:

```
BEFORE fingerprint_store.go            AFTER fingerprint_store.go
───────────────────────────            ──────────────────────────
[]dsp.FingerprintHash (Go)       →     58,000 rows in PostgreSQL
58,000 individual INSERT calls   →     580 batched INSERT calls
Dynamic IN clause construction   →     Single ANY($1) array parameter
Raw sql.Rows iteration           →     []models.HashResult slice
```

---

## What `fingerprint_store.go` Deliberately Does Not Do

| Concern | Handled By |
|---------|-----------|
| Hash generation | `dsp/fingerprint.go` |
| Coherence scoring | `matcher/matcher.go` — next component |
| Song metadata | `store/song_store.go` |
| HTTP responses | `api/handlers/` |
| Connection management | `cmd/api/main.go` |
| Index creation | `db/migrations/002_create_fingerprints.sql` |

---

## Complete Store Layer — Phase 4 Step 2 Final State

```
store/
  ├── song_store.go
  │     Insert, GetByID, GetByFileHash, List, Delete
  │     ErrNotFound, ErrDuplicateHash sentinel errors
  │     Handles CHAR(64) → VARCHAR(64) schema fix
  │
  └── fingerprint_store.go
        InsertBatch   → 580 batched statements for 58,000 rows
        QueryByHashes → ANY($1) array query, 10ms for 1,500 hashes
        CountBySongID → test utility only
```

---

## What Comes Next — Step 3: `matcher/`

With both stores complete, the matcher has everything it needs:

```
matcher/matcher.go
  Receives  → []models.HashResult from fingerprint_store.QueryByHashes
  Does      → builds in-memory time-offset HashMap per song
              calculates coherence scores
              selects Top 3 matches by score
  Returns   → []MatchResult sorted by confidence score
```

> This summary reflects the state of `fingerprint_store.go` and `fingerprint_store_test.go` as committed. Move forward to the matcher part, where we calculate and match Top 3 coherence scores

