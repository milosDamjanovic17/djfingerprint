# Phase 4 — Step 4 — `handlers/` — Introduction Document

---

## What the Handlers Package Is

Handlers are the **meeting point of everything built so far**. Each handler function receives an HTTP request, orchestrates the components beneath it, and writes an HTTP response. No handler contains business logic — they coordinate, not compute.

```
Handler receives request
        │
        ├── dto/request.go      parse and validate the request
        │
        ├── store/              read from or write to the database
        │
        ├── audio/              decode audio (identify only)
        │
        ├── dsp/                generate fingerprint hashes (identify only)
        │
        ├── matcher/            score candidates (identify only)
        │
        └── dto/response.go     build and write the JSON response
```

---

## The Three Files

### `handlers/upload.go` — `POST /api/v1/upload`

Handles full song uploads for fingerprint registration.

**What it does step by step:**

```
1. Parse multipart request          dto.ParseUploadRequest
2. Read file bytes                  io.ReadAll
3. Compute SHA-256 hash             crypto/sha256
4. Check for duplicate              store.GetSongByHash
5. Decode audio                     audio.Decode
6. Compute spectrogram              dsp.ComputeSpectrogram
7. Extract peaks                    dsp.ExtractPeaks
8. Generate fingerprint hashes      dsp.GenerateHashes
9. Insert song record               store.CreateSong
10. Store fingerprint hashes        store.StoreHashes
11. Return 201 Created              dto.NewSongResponse
```

**Error responses:**
```
400 Bad Request     → invalid or unreadable file
409 Conflict        → duplicate file already in library
500 Internal Error  → database or processing failure
```

---

### `handlers/identify.go` — `POST /api/v1/identify`

Handles audio snippet identification — the core DJ fingerprinting use case.

**What it does step by step:**

```
1. Record start time                time.Now()
2. Parse multipart request          dto.ParseIdentifyRequest
3. Read file bytes                  io.ReadAll
4. Decode audio                     audio.Decode
5. Compute spectrogram              dsp.ComputeSpectrogram
6. Extract peaks                    dsp.ExtractPeaks
7. Generate fingerprint hashes      dsp.GenerateHashes
8. Query database for matches       store.QueryByHashes
9. Run coherence scoring            matcher.Match
10. Fetch song metadata             store.GetSongsByIDs
11. Build response                  dto.NewMatchResponse
12. Return 200 OK                   (even when no match found)
```

**Key design decision — 200 even when no match:**
"No match found" is a valid, expected outcome. It is not an error. The response body carries `found: false` to communicate the outcome. A 404 would suggest the route was wrong — not that identification failed.

**Error responses:**
```
400 Bad Request     → invalid or unreadable snippet
500 Internal Error  → database or processing failure
```

---

### `handlers/songs.go` — Song Library Management

Three endpoints for managing the song library:

**`GET /api/v1/songs`** — list all songs with pagination

```
Query params: page (default 1), page_size (default 20, max 100)
Response: dto.ListResponse with songs array and pagination metadata
```

**`GET /api/v1/songs/{id}`** — get a single song by ID

```
URL param: id (integer)
Response: dto.SongResponse
Errors:   400 (invalid id), 404 (not found)
```

**`DELETE /api/v1/songs/{id}`** — remove a song from the library

```
URL param: id (integer)
Response: 204 No Content
Errors:   400 (invalid id), 404 (not found)
```

---

## Handler Structure — Consistent Pattern

Every handler follows the same pattern:

```go
func (h *Handler) MethodName(w http.ResponseWriter, r *http.Request) {
    // 1. Parse input
    // 2. Validate input
    // 3. Call dependencies
    // 4. Handle errors → writeError(w, status, code, message)
    // 5. Write success response → writeJSON(w, status, body)
}
```

Two shared helper functions used by all handlers:

```go
// writeJSON serialises v to JSON and writes it with the given status code
func writeJSON(w http.ResponseWriter, status int, v any) error

// writeError writes a consistent ErrorResponse JSON body
func writeError(w http.ResponseWriter, status int, code, message string)
```

These two functions ensure every response — success or error — has `Content-Type: application/json` and a consistent body shape.

---

## The `Handler` Struct — Dependency Injection

All three handler files share a single `Handler` struct that holds the dependencies:

```go
type Handler struct {
    songStore        store.SongStorer
    fingerprintStore store.FingerprintStorer
}
```

**Why a struct rather than package-level functions:**
Package-level handler functions require package-level variables for dependencies — effectively global state. The struct approach allows multiple instances with different dependencies, makes testing straightforward (inject mock stores), and makes the dependency graph explicit and visible.

**Why interfaces (`SongStorer`, `FingerprintStorer`) not concrete types:**
The handler does not care how songs are stored — PostgreSQL today, something else tomorrow. Testing with a real database in handler tests is slow and requires test fixtures. With interfaces, tests inject lightweight in-memory implementations that return predictable data.

---

## Store Interfaces

Two interfaces defined in `handlers/handler.go`:

```go
type SongStorer interface {
    CreateSong(ctx context.Context, song *models.Song) error
    GetSongByHash(ctx context.Context, hash string) (*models.Song, error)
    GetSongByID(ctx context.Context, id int) (*models.Song, error)
    GetSongs(ctx context.Context, page, pageSize int) ([]*models.Song, int, error)
    DeleteSong(ctx context.Context, id int) error
}

type FingerprintStorer interface {
    StoreHashes(ctx context.Context, hashes []models.FingerprintHash) error
    QueryByHashes(ctx context.Context, hashes []int32) ([]models.HashResult, error)
    DeleteHashesBySongID(ctx context.Context, songID int) error
}
```

---

## Testing Strategy for Handlers

Handler tests use two tools:

**`httptest.NewRecorder()`** — captures the HTTP response without a real server

**In-memory mock stores** — implement the store interfaces with simple map-based storage, returning predictable data without a database connection

```go
// Example mock — returns a fixed song for any ID
type mockSongStore struct {
    songs map[int]*models.Song
}

func (m *mockSongStore) GetSongByID(_ context.Context, id int) (*models.Song, error) {
    song, ok := m.songs[id]
    if !ok {
        return nil, store.ErrNotFound
    }
    return song, nil
}
```

Handler tests verify:
- Correct HTTP status codes
- Correct JSON response shapes
- Correct error responses for bad input
- Correct behaviour when store returns errors

---

## File Structure

```
internal/api/handlers/
  ├── handler.go          ← Handler struct, interfaces, shared helpers
  ├── handler_test.go     ← shared test helpers and mock implementations
  ├── upload.go           ← POST /api/v1/upload
  ├── upload_test.go
  ├── identify.go         ← POST /api/v1/identify
  ├── identify_test.go
  ├── songs.go            ← GET, DELETE /api/v1/songs
  └── songs_test.go
```

---

## What Handlers Deliberately Do Not Do

```
No SQL queries          ← store layer
No DSP algorithms       ← dsp layer
No coherence scoring    ← matcher layer
No audio decoding       ← audio layer
No middleware logic     ← middleware layer
No route registration   ← routes.go
No server startup       ← cmd/api/main.go
```

---


# `handlers/` — Build Summary

---

## What We Built

The `handlers` package is the **meeting point of everything built so far**. Each handler receives an HTTP request, orchestrates the components beneath it, and writes an HTTP response. No handler contains business logic — they coordinate, not compute.

```
HTTP Request
     │
     ▼
dto/request.go       parse and validate
     │
     ├── store/       read / write database
     ├── audio/       decode audio via FFmpeg
     ├── dsp/         spectrogram → peaks → hashes
     └── matcher/     coherence scoring
     │
     ▼
dto/response.go      build JSON response
     │
     ▼
HTTP Response
```

---

## Files Built

```
internal/api/handlers/
  ├── handler.go          shared struct, interfaces, helpers
  ├── handler_test.go     shared mocks
  ├── upload.go           POST /api/v1/upload
  ├── upload_test.go
  ├── identify.go         POST /api/v1/identify
  ├── identify_test.go
  ├── songs.go            GET + DELETE /api/v1/songs
  └── songs_test.go
```

---

## `handler.go` — Shared Foundation

### The `Handler` Struct

```go
type Handler struct {
    songStore        SongStorer
    fingerprintStore FingerprintStorer
}
```

**Why a struct rather than package-level functions:**
Package-level handler functions require package-level variables for dependencies — effectively global state. The struct makes the dependency graph explicit, testable, and replaceable. A test injects a mock store. Production injects a real PostgreSQL store. The handler code is identical in both cases.

---

### The Two Interfaces

```go
type SongStorer interface {
    CreateSong(ctx context.Context, song *models.Song) error
    GetSongByHash(ctx context.Context, hash string) (*models.Song, error)
    GetSongByID(ctx context.Context, id int) (*models.Song, error)
    GetSongs(ctx context.Context, page, pageSize int) ([]*models.Song, int, error)
    DeleteSong(ctx context.Context, id int) error
}

type FingerprintStorer interface {
    StoreFingerprints(ctx context.Context, fingerprints []models.Fingerprint) error
    QueryByHashes(ctx context.Context, hashes []int32) ([]models.HashResult, error)
    DeleteHashesBySongID(ctx context.Context, songID int) error
}
```

**Why interfaces not concrete store types:**
The handler does not care how songs are stored. With interfaces, handler tests inject a lightweight in-memory implementation that returns predictable data in microseconds. No database connection, no test fixtures, no cleanup.

**Why `models.Fingerprint` not `models.FingerprintHash`:**
`models.Fingerprint` is what the models package actually defines — a 32-bit hash, a song ID, and an anchor time. We corrected this from the initial draft when the compiler confirmed the real type name.

---

### `writeJSON` and `writeError`

```go
func writeJSON(w http.ResponseWriter, status int, v any) error {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    return json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
    body := struct {
        Error   string `json:"error"`
        Message string `json:"message"`
    }{Error: code, Message: message}
    writeJSON(w, status, body)
}
```

**Why inline the error shape in `writeError` rather than importing `dto.ErrorResponse`:**
Importing `dto` from `handlers` and having `dto` reference `handlers` types would create an import cycle. The error shape is a two-field struct — simple enough to inline without creating a dependency problem.

**Why `writeJSON` sets the header before encoding:**
`w.WriteHeader` must be called before any write to the body. Once bytes are written, the status code is fixed. Setting `Content-Type` before `WriteHeader` ensures the header is included in the response.

---

### `isNotFound` and `getSongsByIDs`

```go
func isNotFound(err error) bool {
    return err == store.ErrNotFound
}
```

Centralises the `store.ErrNotFound` check. Every handler that needs to distinguish "not found" from "database error" calls this — one place to change if the sentinel error ever moves.

```go
func (h *Handler) getSongsByIDs(ctx context.Context, ids []int) map[int]*models.Song
```

Used by `Identify` to fetch song metadata for all matched song IDs in a single conceptual operation. Songs that cannot be found are silently omitted — the matcher result for a missing song ID is skipped by `dto.NewMatchResponse`.

---

## `upload.go` — `POST /api/v1/upload`

### The Full Pipeline

```
Parse multipart request       dto.ParseUploadRequest
Read file bytes               io.ReadAll
Compute SHA-256 hash          computeFileHash
Check for duplicate           store.GetSongByHash        → 409 if found
Write to temp file            writeTempFile              → FFmpeg needs a path
Decode audio via FFmpeg       audio.DecodeFile
Compute spectrogram           dsp.Compute                → 400 if too short
Extract peaks                 dsp.ExtractPeaks
Generate hashes               dsp.GenerateHashes         → 400 if no peaks
Build song record             buildSongFromRequest
Insert song                   store.CreateSong
Store fingerprints            store.StoreFingerprints
Return 201 Created            dto.NewSongResponse
```

---

### The Temp File Pattern

```go
tmpFile, err := writeTempFile(fileBytes)
if err != nil { ... }
defer os.Remove(tmpFile)

pcm, err := audio.DecodeFile(tmpFile, "")
```

**Why we write to a temp file:**
`audio.DecodeFile` invokes FFmpeg as an external process. FFmpeg takes a file path as input — it cannot read from a Go `io.Reader`. The uploaded bytes must be materialised on disk before FFmpeg can process them. `defer os.Remove` guarantees cleanup regardless of how the handler exits.

**Why `os.CreateTemp` not a fixed path:**
Fixed paths cause race conditions under concurrent requests — two simultaneous uploads would overwrite each other's temp files. `os.CreateTemp` generates a unique path per call using the OS's secure temp file facility.

---

### Duplicate Detection Before FFmpeg

```go
// Step 3 — hash
fileHash := computeFileHash(fileBytes)

// Step 4 — duplicate check
existing, err := h.songStore.GetSongByHash(ctx, fileHash)
if existing != nil {
    writeError(w, http.StatusConflict, dto.ErrCodeDuplicateFile, ...)
    return
}

// Step 5 — temp file + FFmpeg (only reached if not duplicate)
```

**Why hash before FFmpeg:**
FFmpeg decoding is expensive — typically 100–500ms. The SHA-256 hash is computed in microseconds. Rejecting a duplicate before invoking FFmpeg avoids unnecessary processing. A duplicate upload is detected and rejected in under 1ms.

---

### Silence Is a 400, Not a 500

```go
queryHashes, err := dsp.GenerateHashes(peaks, 0)
if err != nil || len(queryHashes) == 0 {
    writeError(w, http.StatusBadRequest, dto.ErrCodeInvalidFile,
        "audio file is too short or contains no usable audio")
    return
}
```

**Why 400 not 500:**
A file that produces no fingerprint hashes is not a server failure — it is an invalid input. Pure silence, a 0.1-second clip, and white noise are all user errors. The server processed the request correctly and determined the file has no usable content. 400 is the correct signal.

---

### `buildSongFromRequest`

```go
func buildSongFromRequest(req *dto.UploadRequest, fileHash string) *models.Song {
    song := &models.Song{
        FileHash: fileHash,
        Source:   "manual",
        ...
    }
    if req.HasTitle()  { song.Title  = &req.Title  }
    if req.HasArtist() { song.Artist = &req.Artist }
    if req.HasAlbum()  { song.Album  = &req.Album  }
    return song
}
```

Extracted from the handler for testability. The `buildSongFromRequest` tests verify metadata handling without touching FFmpeg or the database — pure logic, no dependencies.

---

## `identify.go` — `POST /api/v1/identify`

### The Full Pipeline

```
Record start time             time.Now()
Parse multipart request       dto.ParseIdentifyRequest
Read file bytes               io.ReadAll
Write to temp file            writeTempFile
Decode audio via FFmpeg       audio.DecodeFile          → 400 if invalid
Compute spectrogram           dsp.Compute               → 200 no-match if too short
Extract peaks                 dsp.ExtractPeaks
Generate hashes               dsp.GenerateHashes        → 200 no-match if no peaks
Query database                store.QueryByHashes        → 500 if DB error
Run coherence scoring         matcher.Match
Fetch song metadata           getSongsByIDs
Build response                dto.NewMatchResponse
Return 200 OK                 (always — even for no match)
```

---

### Why 200 Even When No Match Is Found

```go
// "no match" is not an error — it is a valid identification outcome
writeJSON(w, http.StatusOK, dto.NewMatchResponse(nil, nil, time.Since(start)))
```

**Three situations return 200 with `found: false`:**

| Situation | Meaning |
|-----------|---------|
| Spectrogram fails (`dsp.Compute` error) | Snippet too short — valid input, unidentifiable |
| `GenerateHashes` returns empty | Snippet has no distinguishable peaks — featureless audio |
| Matcher returns no results above threshold | No song in the library matches the snippet |

All three are correct outcomes from the server's perspective. The server processed the request successfully. `found: false` communicates the result. A 404 would imply the route does not exist. A 400 would imply the client made a mistake. Neither is accurate.

---

### Silence Is Not a Server Error

One of the key corrections made during testing:

```go
// Before fix — wrong
queryHashes, err := dsp.GenerateHashes(peaks, 0)
if err != nil {
    writeError(w, http.StatusInternalServerError, ...)  // ← wrong
}

// After fix — correct
queryHashes, err := dsp.GenerateHashes(peaks, 0)
if err != nil || len(queryHashes) == 0 {
    writeJSON(w, http.StatusOK, dto.NewMatchResponse(nil, nil, ...))  // ← no match
    return
}
```

A snippet with no detectable peaks is a valid audio file that happens to be unidentifiable. Returning 500 would cause the frontend to show a generic error. Returning `found: false` gives the frontend exactly the information it needs to show "no match found".

---

## `songs.go` — Song Library Management

### `ListSongs` — `GET /api/v1/songs`

```go
page     := parseIntParam(r, "page",      defaultPage)     // default 1
pageSize := parseIntParam(r, "page_size", defaultPageSize) // default 20
```

**Page size clamping:**

```go
if page < 1                          { page = defaultPage }
if pageSize < 1 || pageSize > maxPageSize { pageSize = defaultPageSize }
```

Invalid pagination parameters produce the default rather than an error. A `page_size` of 500 would return the entire library in one response — clamping to 100 protects database performance without rejecting the request.

---

### `GetSong` — `GET /api/v1/songs/{id}`

```go
id, ok := parseSongID(w, r)
if !ok { return }
```

`parseSongID` validates the `{id}` URL parameter. Any non-integer or value less than 1 returns 400 immediately. This prevents the store layer from ever receiving an invalid ID.

---

### `DeleteSong` — `DELETE /api/v1/songs/{id}`

```go
// Delete fingerprints first — maintain referential integrity
h.fingerprintStore.DeleteHashesBySongID(ctx, id)

// Then delete the song record
h.songStore.DeleteSong(ctx, id)
```

**Why fingerprints before song:**
The `fingerprints` table has a foreign key relationship with the `songs` table. Deleting the song record first would leave orphaned fingerprint rows — or fail with a foreign key constraint violation depending on the database schema. Fingerprints are always deleted first.

**Why check existence before deletion:**
The handler calls `GetSongByID` before attempting deletion to return a clean 404 when the song does not exist. Without this check, a `DELETE /api/v1/songs/999` for a non-existent song would return 204 — which implies success. 404 is the correct and honest response.

---

## Testing Strategy

### The Two-Layer Approach

```
Unit tests          No FFmpeg, no database, always pass in CI
                    ├── request parsing errors    → 400
                    ├── duplicate detection       → 409
                    ├── store errors              → 500
                    ├── buildSongFromRequest      → correct model
                    ├── computeFileHash           → correct hash
                    ├── pagination clamping       → correct values
                    └── 404 for missing songs     → correct error code

Integration tests   FFmpeg required, gated by ffmpegAvailable() + t.Skip
                    ├── invalid audio bytes       → 400
                    ├── valid WAV through pipeline → 200/201
                    ├── response shape            → all required keys
                    └── store error after decode  → 500
```

**Why `ffmpegAvailable()`:**

```go
func ffmpegAvailable() bool {
    _, err := exec.LookPath("ffmpeg")
    return err == nil
}
```

Integration tests that require FFmpeg call `t.Skip` when FFmpeg is not in PATH. This means the test suite passes cleanly in a CI environment without FFmpeg installed, and runs the full integration tests on a developer machine that has FFmpeg.

---

### The `noisyWAV()` vs `minimalWAV()` Lesson

The initial test helper `minimalWAV()` produced a valid WAV structure containing pure silence — all zero samples. This caused a discovered failure:

```
silence samples
    → spectrogram with no energy
    → no peaks extracted
    → GenerateHashes returns error or empty
    → handler returns 500
    → test expects 200 → FAIL
```

**The fix had two parts:**

1. Handler logic — silence is a valid no-match result, not a server error. `GenerateHashes` failure returns `200 found: false`.

2. Test data — integration tests that need to reach the database query step use `noisyWAV()` — a 440 Hz sine wave that produces real spectral peaks and passes through the full pipeline.

```go
// minimalWAV — silence — use for tests that stop before DSP
// noisyWAV   — 440Hz sine — use for tests that need full pipeline
```

---

### Mock Stores

```go
type mockSongStore struct {
    songs  map[int]*models.Song
    nextID int
    failOn string   // set to method name to simulate DB error
}

type mockFingerprintStore struct {
    fingerprints []models.Fingerprint
    results      []models.HashResult
    failOn       string
}
```

**The `failOn` pattern:**
Setting `failOn = "CreateSong"` causes `mockSongStore.CreateSong` to return `errMock`. This lets a single mock implementation simulate both the happy path and every error path without separate mock types per scenario.

```go
// Test that CreateSong failure returns 500
ss := newMockSongStore()
ss.failOn = "CreateSong"
h := NewHandler(ss, newMockFingerprintStore())
```

---

## Corrections Made During Development

Three categories of errors were caught and corrected:

| Error | Cause | Fix |
|-------|-------|-----|
| `audio.Decode` undefined | Function is `DecodeFile(path, ffmpegPath)` not `Decode(reader)` | Write to temp file, call `DecodeFile` |
| `dsp.ComputeSpectrogram` undefined | Function is `dsp.Compute` not `dsp.ComputeSpectrogram` | Use correct name |
| `dsp.ExtractPeaks` single return | Returns `([]Peak, [][]float64)` — two values | Capture both: `peaks, _ := dsp.ExtractPeaks(spec)` |
| `dsp.GenerateHashes` single return | Returns `([]QueryHash, error)` | Capture both and handle error |
| `models.FingerprintHash` undefined | Type is `models.Fingerprint` | Use correct type throughout |
| `StoreHashes` undefined | Method is `StoreFingerprints` | Interface and mock updated |
| Silence causes 500 | DSP pipeline has no peaks to hash | Treat as no-match, return `200 found: false` |
| `minimalWAV` breaks pipeline tests | Silent WAV produces no peaks | Added `noisyWAV` with 440Hz sine content |

---

## Tests Written

### `handler_test.go` — Shared

| Component | What It Provides |
|-----------|-----------------|
| `mockSongStore` | In-memory `SongStorer` with `failOn` error injection |
| `mockFingerprintStore` | In-memory `FingerprintStorer` with `failOn` error injection |
| `errMock` | Sentinel error for store failure simulation |
| `makeSong` | Builds a `models.Song` for test fixtures |

### `songs_test.go`

| Test | Verifies |
|------|---------|
| `TestListSongs_EmptyLibrary_Returns200` | Empty library returns 200 not 404 |
| `TestListSongs_EmptyLibrary_ReturnsEmptySongsArray` | Songs field is `[]` not null |
| `TestListSongs_WithSongs_ReturnsSongs` | Songs in store appear in response |
| `TestListSongs_ContentTypeJSON` | Content-Type is application/json |
| `TestListSongs_Pagination_DefaultValues` | page=1 page_size=20 by default |
| `TestListSongs_Pagination_CustomValues` | Custom page and page_size respected |
| `TestListSongs_PageSizeExceedsMax_ClampsToDefault` | page_size > 100 clamped to 20 |
| `TestListSongs_StoreError_Returns500` | DB error returns 500 |
| `TestGetSong_ValidID_Returns200` | Existing song returns 200 |
| `TestGetSong_ValidID_ReturnsCorrectSong` | Correct song ID in response |
| `TestGetSong_InvalidID_Returns400` | Non-integer id returns 400 |
| `TestGetSong_NotFound_Returns404` | Missing song returns 404 |
| `TestGetSong_NotFound_ReturnsErrorCode` | Error code is `not_found` |
| `TestGetSong_StoreError_Returns500` | DB error returns 500 |
| `TestGetSong_ZeroID_Returns400` | id=0 returns 400 |
| `TestDeleteSong_ValidID_Returns204` | Successful delete returns 204 |
| `TestDeleteSong_ValidID_RemovesSong` | Song actually removed from store |
| `TestDeleteSong_InvalidID_Returns400` | Non-integer id returns 400 |
| `TestDeleteSong_NotFound_Returns404` | Missing song returns 404 |
| `TestDeleteSong_FingerprintStoreError_Returns500` | Fingerprint delete failure returns 500 |
| `TestDeleteSong_SongStoreError_Returns500` | Song delete failure returns 500 |
| `TestParseIntParam_ValidValue` | Valid integer parsed correctly |
| `TestParseIntParam_MissingParam_ReturnsDefault` | Missing param returns default |
| `TestParseIntParam_InvalidValue_ReturnsDefault` | Non-integer returns default |

### `upload_test.go`

| Test | FFmpeg | Verifies |
|------|--------|---------|
| `TestUpload_NotMultipart_Returns400` | No | Non-multipart returns 400 |
| `TestUpload_MissingFileField_Returns400` | No | Missing file field returns 400 |
| `TestUpload_InvalidAudio_Returns400` | Yes | Garbage bytes return 400 |
| `TestUpload_DuplicateFile_Returns409` | No | Duplicate hash returns 409 |
| `TestUpload_DuplicateFile_ReturnsErrorCode` | No | Error code is `duplicate_file` |
| `TestUpload_HashCheckStoreError_Returns500` | No | Store error on hash check returns 500 |
| `TestUpload_ValidWAV_ReturnsJSON` | Yes | Content-Type is application/json |
| `TestUpload_ValidWAV_NoFileHash_InResponse` | Yes | `file_hash` absent from response |
| `TestUpload_ValidWAV_WithMetadata_MetadataInResponse` | Yes | Title and artist in response |
| `TestBuildSongFromRequest_WithAllMetadata` | No | All metadata fields mapped |
| `TestBuildSongFromRequest_NoMetadata_NilPointers` | No | Nil pointers when metadata absent |
| `TestComputeFileHash_Deterministic` | No | Same data produces same hash |
| `TestComputeFileHash_DifferentData_DifferentHash` | No | Different data produces different hash |
| `TestComputeFileHash_IsHex` | No | Hash is lowercase hex string |

### `identify_test.go`

| Test | FFmpeg | Verifies |
|------|--------|---------|
| `TestIdentify_NotMultipart_Returns400` | No | Non-multipart returns 400 |
| `TestIdentify_MissingFileField_Returns400` | No | Missing file field returns 400 |
| `TestIdentify_InvalidAudio_Returns400` | Yes | Garbage bytes return 400 |
| `TestIdentify_QueryStoreError_Returns500` | Yes | DB error after decode returns 500 |
| `TestIdentify_ValidSnippet_Returns200` | Yes | Valid audio returns 200 |
| `TestIdentify_ValidSnippet_ResponseIsJSON` | Yes | Content-Type is application/json |
| `TestIdentify_NoMatch_FoundIsFalse` | Yes | No match returns `found: false` |
| `TestIdentify_NoMatch_MatchesIsEmptyArray` | Yes | Matches is `[]` not null |
| `TestIdentify_ResponseContainsProcessedIn` | Yes | `processed_in` present |
| `TestIdentify_ResponseShape_AllRequiredKeys` | Yes | All four keys present in response |

---

## Test Results

```
All tests   → PASS
go vet      → clean
go build    → clean
```

---

## What Comes Next — `routes.go`

The handlers exist. The middleware exists. The next step wires them together:

```
internal/api/
  └── routes.go    ← registers all routes on the chi router
                      applies middleware to the router
                      connects Handler methods to URL patterns
```

After `routes.go`, the final step before the server starts is `cmd/api/main.go` — which initialises the database connection, builds the store implementations, builds the Handler, builds the router, and starts listening.

> Summary complete. Move forward to `routes.go` for further development notes.