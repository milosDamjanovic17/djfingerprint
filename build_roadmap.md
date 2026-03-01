# Build Roadmap — Shazam for DJs

## Overall Philosophy

> Build → Test → Verify → Move to next section. No section starts until the previous one is stable.

Build follows — **Database → Backend → Frontend** right order. The database schema is the contract everything else depends on.

---

## The Sections at a Glance

```
Phase 1 │ Database Schema & Migrations
Phase 2 │ Audio Pipeline (FFmpeg + DSP core)
Phase 3 │ Fingerprint Engine (FFT, Peaks, Hashing)
Phase 4 │ API Layer (Go HTTP server)
Phase 5 │ Frontend Core (Vue)
Phase 6 │ Browser Extension
Phase 7 │ Spotify Metadata Integration
```

> Phases 6 and 7 are post-MVP. Everything through Phase 5 is your v1.0.

---

## Phase 1 — Database Schema & Migrations

**What we build:**
- PostgreSQL setup (local + test with SQLite)
- All tables, indexes, constraints
- Migration tooling
- Seed data for testing

### Deliverables

```
/db
  /migrations
    001_create_songs.sql
    002_create_fingerprints.sql
    003_create_song_features.sql   ← nullable, future Spotify data
  /seeds
    seed_test_songs.sql
  schema.sql                       ← full snapshot
```

### Tables to Define

```sql
-- Migration 001
CREATE TABLE songs (
    id            SERIAL PRIMARY KEY,
    title         VARCHAR(255),
    artist        VARCHAR(255),
    album         VARCHAR(255),
    duration_ms   INTEGER,
    source        VARCHAR(50)     NOT NULL DEFAULT 'manual',
    spotify_id    VARCHAR(100),
    file_hash     CHAR(64)        NOT NULL UNIQUE,  -- SHA256, prevents duplicate uploads
    created_at    TIMESTAMP       NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP       NOT NULL DEFAULT NOW()
);

-- Migration 002
-- This table will be the largest by far
CREATE TABLE fingerprints (
    id            BIGSERIAL PRIMARY KEY,
    hash          INTEGER         NOT NULL,
    song_id       INTEGER         NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
    anchor_time   INTEGER         NOT NULL   -- frame offset, not milliseconds
);

-- The single most important index in the entire application
CREATE INDEX idx_fingerprints_hash ON fingerprints(hash);

-- Migration 003 (future-proofing, all columns nullable)
CREATE TABLE song_features (
    id                  SERIAL PRIMARY KEY,
    song_id             INTEGER NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
    spotify_track_id    VARCHAR(100),
    energy              FLOAT,
    danceability        FLOAT,
    valence             FLOAT,
    acousticness        FLOAT,
    instrumentalness    FLOAT,
    liveness            FLOAT,
    speechiness         FLOAT,
    loudness            FLOAT,
    tempo               FLOAT,
    musical_key         INTEGER,   -- 0-11, Spotify key notation
    created_at          TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### Tests for Phase 1

```
✓ All migrations run forward cleanly on a fresh PostgreSQL instance
✓ All migrations run forward on SQLite (test environment)
✓ SHA256 unique constraint on songs.file_hash rejects duplicate inserts
✓ ON DELETE CASCADE removes fingerprints when a song is deleted
✓ idx_fingerprints_hash exists and is confirmed via EXPLAIN ANALYZE
✓ Inserting 500,000 fingerprint rows and querying by hash completes under 100ms
✓ song_features foreign key rejects orphaned song_id
```

> Phase 1 is done when all migrations are idempotent, all constraints are verified, and the bulk insert + hash lookup benchmark passes.

---

## Phase 2 — Audio Pipeline (FFmpeg Core)

**What we build:**
- Go service that accepts a raw audio file
- FFmpeg decode → mono → 11025 Hz → 16-bit PCM
- Normalization
- Output is a clean `[]int16` slice ready for DSP

**No fingerprinting yet.** This phase is purely about getting clean, consistent audio data out the other side regardless of what goes in (MP3, WAV, WebM, OGG).

### Deliverables

```
/internal
  /audio
    decoder.go        ← FFmpeg invocation, returns PCM samples
    decoder_test.go
    normalizer.go     ← amplitude normalization
    normalizer_test.go
```

### Core Logic Sketch

```go
// decoder.go
type PCMData struct {
    Samples    []int16
    SampleRate int     // always 11025 after decode
    Channels   int     // always 1 (mono)
    Duration   time.Duration
}

func Decode(inputPath string) (*PCMData, error) {
    // ffmpeg -i inputPath
    //        -ac 1           (mono)
    //        -ar 11025       (sample rate)
    //        -f s16le        (signed 16-bit little endian PCM)
    //        -acodec pcm_s16le
    //        pipe:1          (stdout)
    cmd := exec.Command("ffmpeg",
        "-i", inputPath,
        "-ac", "1",
        "-ar", "11025",
        "-f", "s16le",
        "-acodec", "pcm_s16le",
        "pipe:1",
    )
    // read stdout into []byte
    // convert []byte to []int16
    // return PCMData
}
```

### Tests for Phase 2

```
✓ MP3 file decodes to correct sample rate (11025 Hz)
✓ WAV stereo file decoded to mono (channel count = 1)
✓ WebM (browser MediaRecorder output) decodes correctly
✓ OGG decodes correctly
✓ Corrupt/empty file returns a typed error, does not panic
✓ Output sample slice length matches expected duration
  (11025 samples = 1 second ± 5% tolerance)
✓ Normalization: peak amplitude after normalization is within
  target range (e.g. 90-100% of int16 max)
✓ Silent audio file (all zeros) is handled without divide-by-zero
✓ Benchmark: 30-second MP3 decodes in under 2 seconds
```

---

## Phase 3 — Fingerprint Engine

**What we build:**
- Function 1: Spectrogram (Hamming window, sliding FFT)
- Function 2: Peak extraction (6 log bands, dynamic threshold)
- Function 3: Hash generation (anchor/target pairs → 32-bit hash)
- Fingerprint store (INSERT to DB)
- Fingerprint query (SELECT + coherence scoring → Top 3)

This is the **heart of the application**. Take the most time here.

### Deliverables

```
/internal
  /dsp
    spectrogram.go        ← FFT, Hamming, sliding window
    spectrogram_test.go
    peaks.go              ← 6-band extraction, dynamic threshold
    peaks_test.go
    fingerprint.go        ← hash generation, anchor/target pairing
    fingerprint_test.go
  /store
    fingerprint_store.go  ← DB insert + query
    fingerprint_store_test.go
  /matcher
    matcher.go            ← coherence scoring, Top 3 selection
    matcher_test.go
```

### Hash Encoding — Locked In

$$\text{hash} = (f_{\text{anchor}} \ll 23) \mid (f_{\text{target}} \ll 14) \mid \Delta t$$

```go
// fingerprint.go
func encodeHash(anchorFreq, targetFreq, deltaTime int) int32 {
    return int32((anchorFreq & 0x1FF) << 23 |
                 (targetFreq & 0x1FF) << 14 |
                 (deltaTime  & 0x3FFF))
}
```

### Frequency Band Map (Hz → Bins at 11025 Hz, FFT size 1024)

$$\text{bin} = \left\lfloor \frac{f \times 1024}{11025} \right\rfloor$$

| Band | Hz Range | Bin Start | Bin End |
|------|----------|-----------|---------|
| Very Low | 20–300 | 1 | 27 |
| Low | 300–600 | 27 | 55 |
| Low-Mid | 600–1200 | 55 | 111 |
| Mid | 1200–2400 | 111 | 222 |
| High-Mid | 2400–4000 | 222 | 371 |
| High | 4000–5000 | 371 | 464 |

### Tests for Phase 3

```
✓ Hamming window output: first and last sample near zero
✓ FFT of a pure 440 Hz sine wave peaks at correct bin
✓ Sliding window produces correct number of frames
  given audio length and hop size
✓ Peak extraction returns exactly 1 peak per band per frame
✓ Dynamic threshold: peaks below average of top 6 are discarded
✓ Hash encoding is deterministic (same input = same hash every time)
✓ Hash decoding recovers original anchorFreq, targetFreq, deltaTime
✓ Fan-out = 5: each anchor generates exactly 5 hashes max
✓ Known song fingerprinted twice produces identical hash sets
✓ Coherence scoring: two identical snippets score 100%
✓ Coherence scoring: completely different audio scores near 0
✓ Top 3 returned in descending coherence order
✓ Query with no DB matches returns empty result, no panic
✓ 10-second snippet matches correctly against full 3-minute song in DB
✓ Benchmark: fingerprint a 3-minute song in under 10 seconds
```

---

## Phase 4 — API Layer (Go HTTP Server)

**What we build:**
- HTTP server (standard `net/http` or `chi` router)
- Endpoints for upload, identify, library management
- File handling (multipart), validation
- Error handling, logging

### Deliverables

```
/cmd
  /api
    main.go
/internal
  /api
    routes.go
    middleware.go          ← logging, recovery, CORS
    /handlers
      identify.go          ← POST /identify
      songs.go             ← GET/DELETE /songs
      upload.go            ← POST /upload
    /dto
      request.go           ← request structs
      response.go          ← response structs
```

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/identify` | Submit audio snippet, returns Top 3 matches |
| `POST` | `/api/v1/upload` | Upload MP3/WAV to index into fingerprint DB |
| `GET` | `/api/v1/songs` | List all indexed songs (paginated) |
| `GET` | `/api/v1/songs/:id` | Get single song metadata |
| `DELETE` | `/api/v1/songs/:id` | Remove song + its fingerprints |
| `GET` | `/api/v1/health` | Health check |

### Tests for Phase 4

```
✓ POST /identify with valid WebM audio returns 200 + Top 3 JSON
✓ POST /identify with empty body returns 400
✓ POST /identify with non-audio file returns 415
✓ POST /upload with valid MP3 returns 201 + song ID
✓ POST /upload same file twice returns 409 (duplicate file_hash)
✓ POST /upload file > size limit returns 413
✓ GET /songs returns paginated list
✓ GET /songs/:id returns 404 for unknown ID
✓ DELETE /songs/:id removes song and all fingerprints (cascade verified)
✓ All endpoints return correct Content-Type: application/json
✓ CORS headers present for browser requests
✓ Malformed JSON body returns 400 with readable message
✓ Server recovers from panics in handlers (middleware test)
✓ Integration test: upload song → identify 10s snippet → correct match returned
```

---

## Phase 5 — Frontend (Vue)

**What we build:**
- Record from microphone
- Record from system audio (tab/desktop via browser API)
- Upload MP3/WAV manually
- Show identification results (Top 3 cards)
- Song library view
- Basic responsive layout

### Deliverables

```
/frontend
  /src
    /components
      AudioRecorder.vue     ← mic + system audio capture
      FileUpload.vue
      MatchResult.vue       ← Top 3 result cards
      SongCard.vue
    /views
      IdentifyView.vue
      LibraryView.vue
    /api
      client.ts             ← typed API calls
    /stores
      songs.ts              ← Pinia store
    App.vue
    main.ts
```

### Tests for Phase 5

```
✓ AudioRecorder captures microphone audio and POSTs to /identify
✓ AudioRecorder captures system audio via getDisplayMedia
✓ Capture stops after configurable duration (default 15 seconds)
✓ FileUpload accepts .mp3 and .wav, rejects other formats client-side
✓ FileUpload shows progress indicator during upload
✓ MatchResult displays Top 3 with title, artist, coherence score
✓ MatchResult handles "no match found" state gracefully
✓ LibraryView paginates song list correctly
✓ Delete song from library triggers confirmation, calls DELETE endpoint
✓ API error responses surface readable messages to the user
✓ Works on Chrome, Firefox, Safari (MediaRecorder compatibility)
✓ Responsive layout on 1024px+ (DJ laptop screen minimum target)
```

---

## Summary — What to Build First

```
START HERE
    │
    ▼
Phase 1: Database          ← 2–3 days
    │  All migration + index tests pass
    ▼
Phase 2: Audio Pipeline    ← 2–3 days
    │  All format decode + normalize tests pass
    ▼
Phase 3: Fingerprint Engine ← 5–7 days (most complex)
    │  Unit + integration tests pass, benchmark passes
    ▼
Phase 4: API Layer         ← 3–4 days
    │  All endpoint tests pass, integration test passes
    ▼
Phase 5: Frontend          ← 4–5 days
    │  All capture + display tests pass
    ▼
MVP COMPLETE
    │
    ├── Phase 6: Browser Extension
    └── Phase 7: Spotify Features
```

> **Start with Phase 1.** When you're ready, share what you have and we'll build the migrations and tests together step by step.