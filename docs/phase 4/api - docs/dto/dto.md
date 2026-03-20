# Phase 4 — Step 4 — `dto/` — Introduction Document

---

## What the `dto` Package Is

DTO stands for **Data Transfer Object**. The `dto` package defines the exact JSON shapes that travel across the HTTP boundary — what the API accepts in request bodies and what it sends back in response bodies.

```
Client (Vue frontend)
      │
      │  JSON request body
      ▼
dto/request.go    ← defines what we accept
      │
      ▼
handlers/         ← business logic
      │
      ▼
dto/response.go   ← defines what we send back
      │
      │  JSON response body
      ▼
Client (Vue frontend)
```

---

## Why a Separate `dto` Package

Without DTOs, two bad things happen:

### Problem 1 — Models leak into the API

```go
// Without DTOs — handler returns a raw models.Song directly
json.NewEncoder(w).Encode(song)
```

This exposes every field of `models.Song` including `FileHash` — an internal implementation detail that the frontend has no use for and should never see. The database representation and the API representation are conflated.

### Problem 2 — No stable API contract

If `models.Song` gains a new field for an internal purpose, it automatically appears in the API response — potentially breaking the frontend without any intentional API change.

DTOs give you an explicit, stable contract:

```
models.Song     →  internal representation  →  full database mirror
dto.SongResponse →  external representation →  only what the client needs
```

---

## The Two Files

### `dto/request.go`

Defines what the API accepts from clients.

**Why requests need DTOs:**
Multipart file uploads and JSON bodies need to be parsed and validated before the handler can use them. The request DTOs define the expected shape and carry validation rules.

```
UploadRequest
  ├── File        multipart.File     — the audio file bytes
  ├── FileHeader  multipart.FileHeader — filename, size, content type
  ├── Title       string (optional)  — track title if known
  ├── Artist      string (optional)  — artist name if known
  └── Album       string (optional)  — album name if known

IdentifyRequest
  └── File        multipart.File     — the audio snippet bytes
      FileHeader  multipart.FileHeader — snippet metadata
```

Both requests are multipart form uploads — not JSON bodies. Audio files cannot be sent as JSON. The `Content-Type` must be `multipart/form-data`.

---

### `dto/response.go`

Defines what the API sends back to clients.

**Four response types:**

```
SongResponse
  ├── ID          int
  ├── Title       *string
  ├── Artist      *string
  ├── Album       *string
  ├── DurationMs  *int
  ├── Source      string
  ├── SpotifyID   *string
  └── CreatedAt   time.Time
  (FileHash deliberately excluded — internal implementation detail)

MatchResponse
  ├── Found       bool
  ├── Confidence  string        — "high" / "medium" / "low"
  ├── Matches     []MatchItem
  └── ProcessedIn string        — e.g. "1.23s"

MatchItem
  ├── Rank        int           — 1, 2, or 3
  ├── Song        SongResponse  — the candidate song
  ├── Score       int           — raw coherence score
  └── OffsetSecs  float64       — where in the song the snippet came from

ErrorResponse
  ├── Error   string  — machine-readable error code
  └── Message string  — human-readable description
```

---

## The Confidence String — Why Not Just Return the Score

The raw coherence score (e.g. 847) is meaningless to a frontend developer or end user. They need to know: **how confident should the user be in this result?**

Three confidence levels with defined thresholds:

| Threshold | Confidence | Meaning |
|-----------|-----------|---------|
| Score ≥ 50 | `"high"` | Very likely correct — present to user as definitive |
| Score ≥ 15 | `"medium"` | Probably correct — present with mild uncertainty |
| Score ≥ 5 | `"low"` | Possible match — present as a suggestion only |

These thresholds are defined as constants in `response.go` so they can be adjusted independently of any handler logic.

---

## What the API Responses Look Like

### Successful Upload — `POST /api/v1/upload` → `201 Created`

```json
{
  "id": 42,
  "title": "Knock Knock",
  "artist": "DJ Koze",
  "source": "manual",
  "created_at": "2024-01-15T14:23:01Z"
}
```

### Successful Identification — `POST /api/v1/identify` → `200 OK`

```json
{
  "found": true,
  "confidence": "high",
  "processed_in": "1.23s",
  "matches": [
    {
      "rank": 1,
      "score": 847,
      "offset_secs": 45.14,
      "song": {
        "id": 42,
        "title": "Knock Knock",
        "artist": "DJ Koze",
        "source": "manual",
        "created_at": "2024-01-15T14:23:01Z"
      }
    }
  ]
}
```

### No Match Found — `POST /api/v1/identify` → `200 OK`

```json
{
  "found": false,
  "confidence": "",
  "processed_in": "0.98s",
  "matches": []
}
```

### Error Response — any error → `4xx` or `5xx`

```json
{
  "error": "duplicate_file",
  "message": "A song with this file already exists in the library"
}
```

---

## Error Codes — Defined as Constants

Machine-readable error codes allow the frontend to handle specific errors programmatically rather than parsing error message strings:

```go
const (
    ErrCodeDuplicateFile    = "duplicate_file"
    ErrCodeInvalidFile      = "invalid_file"
    ErrCodeFileTooLarge     = "file_too_large"
    ErrCodeNotFound         = "not_found"
    ErrCodeInternalError    = "internal_error"
    ErrCodeNoMatch          = "no_match"
    ErrCodeInvalidRequest   = "invalid_request"
)
```

The frontend can show different UI states for `duplicate_file` vs `internal_error` without doing string matching on the message field.

---

## Helper Functions in `response.go`

Three constructor functions that build response objects from internal types:

```
NewSongResponse(song *models.Song) SongResponse
    Converts models.Song → SongResponse
    Deliberately excludes FileHash

NewMatchResponse(results []matcher.MatchResult, songs map[int]*models.Song, duration time.Duration) MatchResponse
    Builds the full identification response
    Calculates confidence from top result score
    Formats duration as human-readable string

NewErrorResponse(code, message string) ErrorResponse
    Builds a consistent error response
```

---

## What Tests Will Cover

```
[ ] SongResponse excludes FileHash field
[ ] SongResponse includes all expected fields
[ ] SongResponse JSON field names are snake_case
[ ] Nil pointer fields omitted from SongResponse JSON
[ ] MatchResponse found=true when matches present
[ ] MatchResponse found=false when matches empty
[ ] Confidence level calculated correctly for all thresholds
[ ] Ranks are 1, 2, 3 (not 0, 1, 2)
[ ] ProcessedIn formatted correctly
[ ] ErrorResponse error code and message correct
[ ] All error code constants defined
[ ] NewSongResponse converts models.Song correctly
[ ] NewMatchResponse builds correct structure
[ ] JSON marshal produces correct output for all types
```

---

## File Location and Package

```
internal/api/dto/
  ├── request.go
  ├── request_test.go
  ├── response.go
  └── response_test.go

package dto
```

---

# `dto/` — Build Summary

---

## What We Built

### The Purpose
The `dto` package is the **explicit contract between the API and its clients**. It defines the exact JSON shapes that travel across the HTTP boundary in both directions — what the server accepts and what it sends back. Before DTOs existed, the boundary between internal data structures and external API responses was implicit and fragile. After DTOs, the contract is explicit, stable, and independently testable.

---

## The Core Problem DTOs Solve

```
Without DTOs                          With DTOs
────────────────────────────────      ────────────────────────────────
models.Song returned directly         SongResponse returned
FileHash visible to clients           FileHash never leaves the server
API changes when model changes        API changes only when DTO changes
No stable contract                    Explicit, versioned contract
Handler logic mixed with shape        Shape defined separately
```

The most important example — `FileHash`:

```go
// models.Song — internal representation
type Song struct {
    ID       int
    Title    *string
    FileHash string    // ← internal — used for duplicate detection
    Source   string
    // ...
}

// dto.SongResponse — external representation
type SongResponse struct {
    ID     int     `json:"id"`
    Title  *string `json:"title,omitempty"`
    Source string  `json:"source"`
    // FileHash deliberately absent
}
```

`FileHash` is a SHA-256 hash of the audio file used internally to detect duplicate uploads. It has no meaning to the frontend and must never appear in API responses. The DTO layer enforces this at the type level — there is no way to accidentally include it.

---

## `request.go` — What Was Built and Why

### Constants

```go
const (
    MaxUploadSize  = 500 << 20  // 500MB
    MaxSnippetSize = 10 << 20   // 10MB
)
```

**Why `500 << 20` rather than `524288000`:**
Bit-shifting is idiomatic Go for byte size constants. `500 << 20` is self-documenting — it reads as "500 times 2²⁰" which is exactly 500 mebibytes. The intent is clear without a comment.

**Why 500MB for uploads:**
A 24-bit 96kHz stereo WAV file (studio quality) runs approximately 1GB per hour. A 30-minute DJ set excerpt at CD quality (16-bit 44.1kHz stereo) is approximately 300MB. 500MB covers the largest realistic single track upload while protecting against accidental multi-gigabyte submissions.

**Why 10MB for snippets:**
A 15-second audio clip at 128kbps MP3 is approximately 240KB. Even at uncompressed WAV quality a 15-second snippet is under 3MB. 10MB is deliberately generous to handle any format without rejecting valid clips.

---

### `UploadRequest`

```go
type UploadRequest struct {
    File   multipart.File
    Header *multipart.FileHeader
    Title  string
    Artist string
    Album  string
}
```

**Why `multipart.File` and `*multipart.FileHeader` rather than `[]byte`:**
Reading the entire file into `[]byte` before returning from the parser would load potentially 500MB into memory at once. `multipart.File` is an `io.Reader` — the audio data can be streamed from the request body directly through the decoder pipeline without buffering the entire file.

**Why Title, Artist, Album are plain `string` not `*string`:**
In the request context an empty string and a missing field are semantically identical — the user simply did not provide the metadata. `*string` would add nil-checking complexity with no benefit. The `HasTitle()`, `HasArtist()`, `HasAlbum()` helper methods check for empty string, which is sufficient.

---

### `ParseUploadRequest` and `ParseIdentifyRequest`

Both parsers apply `http.MaxBytesReader` before calling `ParseMultipartForm`:

```go
r.Body = http.MaxBytesReader(nil, r.Body, MaxUploadSize)
if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
    return nil, fmt.Errorf("request body too large or not multipart: %w", err)
}
```

**Why `MaxBytesReader` before `ParseMultipartForm`:**
Without `MaxBytesReader`, `ParseMultipartForm` would happily read the entire request body regardless of size — a 10GB upload would consume 10GB of server memory before returning an error. `MaxBytesReader` wraps the body with a hard limit — reading stops and returns an error the moment the limit is exceeded.

**Why `%w` error wrapping:**
Wrapping with `%w` preserves the original error for callers who want to use `errors.Is` or `errors.As` to inspect the underlying cause. Handlers can check whether the error is a size limit violation vs a missing field and return the appropriate HTTP status code.

---

### Helper Methods on `UploadRequest`

```go
func (r *UploadRequest) HasTitle() bool   { return r.Title != "" }
func (r *UploadRequest) HasArtist() bool  { return r.Artist != "" }
func (r *UploadRequest) HasAlbum() bool   { return r.Album != "" }
func (r *UploadRequest) Filename() string { ... }
func (r *UploadRequest) FileSize() int64  { ... }
```

These exist so handlers read cleanly:

```go
// With helpers — readable
if req.HasTitle() {
    song.Title = &req.Title
}

// Without helpers — repetitive
if req.Title != "" {
    song.Title = &req.Title
}
```

`Filename()` and `FileSize()` guard against nil `Header` — if somehow `Header` is nil the methods return zero values rather than panicking.

---

## `response.go` — What Was Built and Why

### Confidence Thresholds

```go
const (
    ConfidenceHighThreshold   = 50
    ConfidenceMediumThreshold = 15
)
```

| Score | Confidence | Meaning |
|-------|-----------|---------|
| ≥ 50 | `"high"` | Very likely correct — 50 coherent hash pairs is extremely unlikely by chance |
| ≥ 15 | `"medium"` | Probably correct — degraded audio or background noise |
| ≥ 5 | `"low"` | Possible match — heavy degradation or very short snippet |

**Why thresholds are constants not magic numbers:**
The frontend logic can reference `ConfidenceHighThreshold` if it ever needs to display a numeric boundary. More importantly, changing the thresholds is a one-line edit in one place — not a search-and-replace across handler logic.

**Why three levels not a percentage:**
A percentage implies mathematical precision that does not exist in the coherence scoring algorithm. "High / medium / low" is honest about what the score actually means — a rough signal strength indicator, not a calibrated probability.

---

### Error Code Constants

```go
const (
    ErrCodeDuplicateFile  = "duplicate_file"
    ErrCodeInvalidFile    = "invalid_file"
    ErrCodeFileTooLarge   = "file_too_large"
    ErrCodeNotFound       = "not_found"
    ErrCodeInternalError  = "internal_error"
    ErrCodeNoMatch        = "no_match"
    ErrCodeInvalidRequest = "invalid_request"
)
```

**Why machine-readable codes alongside human-readable messages:**
The `message` field is for display. The `error` field is for logic. A Vue component can do:

```javascript
if (response.error === 'duplicate_file') {
    showDuplicateWarning()
} else if (response.error === 'file_too_large') {
    showSizeWarning()
}
```

Without the `error` code field the frontend would have to parse the `message` string — fragile and breaks whenever the message wording changes.

---

### `SongResponse`

```go
type SongResponse struct {
    ID         int       `json:"id"`
    Title      *string   `json:"title,omitempty"`
    Artist     *string   `json:"artist,omitempty"`
    Album      *string   `json:"album,omitempty"`
    DurationMs *int      `json:"duration_ms,omitempty"`
    Source     string    `json:"source"`
    SpotifyID  *string   `json:"spotify_id,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
}
```

**Why pointer fields with `omitempty`:**
`Title`, `Artist`, `Album`, `DurationMs`, and `SpotifyID` are all optional — a manually uploaded track may have none of them. Using `*string` with `omitempty` means these fields simply do not appear in the JSON when nil. The frontend does not need to handle `null` values for every optional field.

**Why `CreatedAt` is not omitempty:**
`CreatedAt` is always populated — every song has a creation time. Making it omitempty would be misleading.

**Why `Source` is not a pointer:**
`Source` is always either `"manual"` or `"spotify"`. It is never absent. A plain `string` is correct here.

---

### `SongTitle()` and `SongArtist()` Helper Methods

```go
func (s SongResponse) SongTitle() string {
    if s.Title == nil { return "" }
    return *s.Title
}
```

Safe dereferencing of pointer fields. Used in tests and in any handler code that needs to read these values back from a response object without nil-checking at every call site.

---

### `MatchResponse` and `MatchItem`

```go
type MatchResponse struct {
    Found       bool        `json:"found"`
    Confidence  string      `json:"confidence"`
    Matches     []MatchItem `json:"matches"`
    ProcessedIn string      `json:"processed_in"`
}

type MatchItem struct {
    Rank       int          `json:"rank"`
    Song       SongResponse `json:"song"`
    Score      int          `json:"score"`
    OffsetSecs float64      `json:"offset_secs"`
}
```

**Why `Found bool` when `len(Matches) > 0` carries the same information:**
Frontend code reads more clearly with an explicit `found` flag. `if (response.found)` is unambiguous. `if (response.matches.length > 0)` requires the developer to understand the implicit contract.

**Why `Rank` starts at 1 not 0:**
Users and frontend displays think in terms of "first match", "second match" — 1-indexed. The 0-indexed slice position is an implementation detail that does not belong in the API contract.

**Why `Score` is included in `MatchItem`:**
The raw coherence score is included for advanced clients — a DJ application might want to show different UI treatment for a score of 847 vs a score of 12, both of which might be "low" confidence. The frontend has access to the raw signal if it wants it.

**Why `OffsetSecs` is included:**
For a DJ context knowing where in the full track a snippet came from is valuable — "this snippet is from 1 minute 37 seconds into the track" helps the DJ understand context.

---

### `NewMatchResponse`

```go
func NewMatchResponse(
    results []matcher.MatchResult,
    songs map[int]*models.Song,
    duration time.Duration,
) MatchResponse
```

**Why a `songs map[int]*models.Song` parameter:**
The matcher returns `[]MatchResult` with song IDs and scores. It does not have access to song metadata — that is the store layer's responsibility. The handler retrieves metadata for the matched song IDs and passes the map here. This keeps the matcher pure (no database dependency) and the response builder focused (no business logic).

**Why `Matches` is initialised as `[]MatchItem{}` not nil:**
When no results are found, `Matches` must encode as `[]` in JSON not `null`. JavaScript `response.matches.forEach(...)` fails if `matches` is `null`. Initialising as an empty slice guarantees `[]` in the JSON output regardless of whether any matches were added.

**Missing song metadata handling:**
```go
song, ok := songs[result.SongID]
if !ok {
    continue // skip — do not include partial data
}
```

If the handler fails to retrieve metadata for a matched song ID, that match is silently skipped rather than including a `MatchItem` with empty song fields. A partial response is worse than a shorter accurate response.

---

### `formatProcessingTime`

```go
func formatProcessingTime(d time.Duration) string {
    if d < time.Second {
        return fmt.Sprintf("%dms", d.Milliseconds())
    }
    return fmt.Sprintf("%.2fs", d.Seconds())
}
```

Mirrors the `formatDuration` logic from the logging middleware. Sub-second operations show milliseconds for precision. The identification pipeline typically completes in 200–800ms — milliseconds is the right unit for that range.

---

## Tests Written

### `request_test.go`

| Test | What It Verifies |
|------|-----------------|
| `TestParseUploadRequest_Success` | File and header populated from valid multipart request |
| `TestParseUploadRequest_WithMetadata` | Title, artist, album parsed from form fields |
| `TestParseUploadRequest_MissingFile_ReturnsError` | Missing file field returns error |
| `TestParseUploadRequest_NotMultipart_ReturnsError` | JSON body returns error |
| `TestParseUploadRequest_EmptyMetadata_IsAllowed` | Missing optional fields produce empty strings |
| `TestUploadRequest_HasTitle_True` | HasTitle returns true for non-empty title |
| `TestUploadRequest_HasTitle_False` | HasTitle returns false for empty title |
| `TestUploadRequest_HasArtist_True` | HasArtist returns true for non-empty artist |
| `TestUploadRequest_HasArtist_False` | HasArtist returns false for empty artist |
| `TestUploadRequest_HasAlbum_True` | HasAlbum returns true for non-empty album |
| `TestUploadRequest_HasAlbum_False` | HasAlbum returns false for empty album |
| `TestUploadRequest_Filename` | Filename returns header filename |
| `TestUploadRequest_Filename_NilHeader` | Filename returns empty for nil header |
| `TestUploadRequest_FileSize` | FileSize returns header size |
| `TestUploadRequest_FileSize_NilHeader` | FileSize returns 0 for nil header |
| `TestParseIdentifyRequest_Success` | File and header populated from valid snippet request |
| `TestParseIdentifyRequest_MissingFile_ReturnsError` | Missing file field returns error |
| `TestParseIdentifyRequest_NotMultipart_ReturnsError` | Non-multipart body returns error |
| `TestIdentifyRequest_Filename` | Filename returns header filename |
| `TestIdentifyRequest_Filename_NilHeader` | Filename returns empty for nil header |
| `TestConstants_MaxUploadSize` | MaxUploadSize equals 500MB |
| `TestConstants_MaxSnippetSize` | MaxSnippetSize equals 10MB |

### `response_test.go`

| Test | What It Verifies |
|------|-----------------|
| `TestNewSongResponse_FieldsPopulated` | ID, title, artist, source mapped correctly |
| `TestNewSongResponse_FileHash_Excluded` | file_hash absent from JSON output |
| `TestNewSongResponse_NilFields_OmittedFromJSON` | nil pointer fields absent from JSON |
| `TestNewSongResponse_JSON_FieldNames` | id, source, created_at present with correct names |
| `TestSongResponse_SongTitle_NilReturnsEmpty` | nil Title pointer returns empty string |
| `TestSongResponse_SongTitle_NonNilReturnsValue` | non-nil Title pointer returns value |
| `TestSongResponse_SongArtist_NilReturnsEmpty` | nil Artist pointer returns empty string |
| `TestSongResponse_SongArtist_NonNilReturnsValue` | non-nil Artist pointer returns value |
| `TestScoreToConfidence_High` | Scores ≥ 50 return "high" |
| `TestScoreToConfidence_Medium` | Scores 15–49 return "medium" |
| `TestScoreToConfidence_Low` | Scores 5–14 return "low" |
| `TestScoreToConfidence_ExactBoundaries` | Boundary values correct for all thresholds |
| `TestFormatProcessingTime_Milliseconds` | Sub-second duration formats as ms |
| `TestFormatProcessingTime_Seconds` | Second+ duration formats with 2 decimal places |
| `TestFormatProcessingTime_ExactlyOneSecond` | Exactly 1s formats as "1.00s" |
| `TestNewMatchResponse_NoResults_FoundIsFalse` | Found=false for empty results |
| `TestNewMatchResponse_NoResults_MatchesIsEmptySlice` | Matches is [] not nil for empty results |
| `TestNewMatchResponse_NoResults_ConfidenceIsEmpty` | Confidence="" for empty results |
| `TestNewMatchResponse_NoResults_ProcessedInPopulated` | ProcessedIn populated even with no results |
| `TestNewMatchResponse_WithResults_FoundIsTrue` | Found=true when results present |
| `TestNewMatchResponse_WithResults_MatchCount` | Correct number of matches in response |
| `TestNewMatchResponse_WithResults_Ranks_OneIndexed` | Ranks are 1, 2, 3 not 0, 1, 2 |
| `TestNewMatchResponse_WithResults_ConfidenceHigh` | High confidence for score at threshold |
| `TestNewMatchResponse_WithResults_OffsetSecs` | OffsetSecs value propagated correctly |
| `TestNewMatchResponse_MissingSongMetadata_SkipsResult` | Missing song metadata skips match |
| `TestNewErrorResponse_FieldsSet` | Error code and message set correctly |
| `TestNewErrorResponse_JSON` | JSON output contains correct field names and values |
| `TestErrorCodeConstants_Defined` | All seven error code constants non-empty |
| `TestNewListResponse_FieldsSet` | Songs, total, page, page_size set correctly |
| `TestNewListResponse_EmptySongs` | Empty song list handled correctly |
| `TestNewListResponse_FileHash_ExcludedFromSongs` | file_hash absent from list response songs |
| `TestMatchResponse_JSON_Shape` | found, confidence, matches, processed_in all present |
| `TestMatchResponse_EmptyMatches_JSON_IsArray` | Empty matches encodes as [] not null |

---

## What DTOs Deliberately Do Not Do

| Concern | Handled By |
|---------|-----------|
| Business logic | `api/handlers/` |
| Database access | `store/` |
| File parsing | `audio/` |
| Route matching | `api/routes.go` |
| Authentication | Phase 7 |
| Input validation beyond parsing | `api/handlers/` |

---

## Test Results Achieved

```
All tests   → PASS
go vet      → clean
go build    → clean
```

---

## What Comes Next — `handlers/`

With middleware and DTOs complete, the handlers have everything they need:

```
internal/api/handlers/
  ├── upload.go    ← POST /api/v1/upload
  ├── identify.go  ← POST /api/v1/identify
  └── songs.go     ← GET /api/v1/songs, DELETE /api/v1/songs/{id}
```

Each handler:
- Parses the request using `dto/request.go`
- Calls the appropriate store methods
- Runs the DSP pipeline (identify only)
- Calls the matcher (identify only)
- Builds the response using `dto/response.go`
- Returns the appropriate HTTP status code

> This summary reflects the state of `dto/request.go` and `dto/response.go` as committed. Move forward to `handlers/` for further development docs.