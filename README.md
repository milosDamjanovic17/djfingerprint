# DJ Fingerprint

A Shazam-like music recognition app for DJs. You build a library by uploading full tracks. Each track is turned into a set of audio fingerprints and stored in PostgreSQL. Later you can record a short snippet, either through the microphone or by capturing audio from a browser tab, and the app tells you which track in your library it is and roughly where in the track the snippet came from.

- **Backend:** Go (chi router, FFmpeg for decoding, gonum FFT, PostgreSQL)
- **Frontend:** Vue 3 + TypeScript (Vite, Pinia, Tailwind CSS 4)

---

## Using the app

The app has three pages, reachable from the top navigation bar.

### 1. Upload (`/`)
1. Drag and drop an audio file onto the upload zone, or click it to choose a file. Supported formats are `.mp3`, `.wav`, `.flac`, `.m4a` and `.ogg`, up to 500 MB.
2. The backend decodes the file, fingerprints it, and saves it to the library.
3. Title, artist and album come from the first source that has them:
   1. values entered in the form,
   2. tags embedded in the file (ID3, FLAC, M4A, OGG),
   3. the file name, if it follows the `Artist - Title.ext` convention (common for Beatport-style DJ files that have no tags).
4. If you upload the exact same file twice, it is rejected as a duplicate (`409 Conflict`). Duplicates are detected by the file's SHA-256 hash.

### 2. Identify (`/identify`)
1. Choose an audio source with the toggle:
   - **Microphone:** listens to the room, for example a track playing on speakers.
   - **Browser Audio:** captures audio from a browser tab (Chrome/Edge only). When the share dialog opens, pick the tab and tick **"Share tab audio"**. Video is discarded; only the audio is recorded.
2. Press the record button. The app records **10 seconds**, then sends the clip to the backend.
3. The result shows the best match with its title, artist, duration, a confidence bar, and the position in the track the snippet was matched to. If nothing in the library is a strong enough match, the app shows **"No match"**.

### 3. Library (`/library`)
Shows a paginated list of every indexed track. You can delete a track from here, which also deletes all of its fingerprints.

### Using it from a phone
The Vite dev server runs over HTTPS and is reachable on your local network. Open `https://<your-machine-ip>:5173` on the phone and accept the self-signed certificate warning (**Advanced → Proceed**). HTTPS is required because browsers only allow microphone access (`getUserMedia`) and tab audio capture (`getDisplayMedia`) on secure origins.

---

## How it works

### Overview

```
                ┌──────────── Upload (full song) ────────────┐
audio file ──►  FFmpeg decode ──► spectrogram ──► peaks ──► hashes ──► INSERT into fingerprints
                (11025 Hz mono)    (FFT)          (6 bands)  (pairs)

                ┌──────────── Identify (10s snippet) ────────┐
snippet    ──►  FFmpeg decode ──► spectrogram ──► peaks ──► hashes ──► SELECT matching hashes
                                                                            │
                                                     time-offset histogram ◄┘
                                                                            │
                                                     top 3 songs + confidence
```

Uploads and identification share the same fingerprinting pipeline. The difference is what happens at the end: an upload **stores** the hashes, while identification **looks them up**.

### 1. Audio decoding (`internal/audio`)
FFmpeg converts any input format to **16-bit PCM, mono, 11025 Hz**. Then the signal is peak-normalized. A low sample rate is enough here because the fingerprinting only looks at frequencies up to 5 kHz, and it keeps processing fast.

### 2. Spectrogram (`internal/dsp/spectrogram.go`)
The PCM signal is split into overlapping windows: **1024 samples per window, a hop of 512 samples** (50% overlap, about 46 ms per frame). Each window gets a Hamming window and an FFT. The result is a time × frequency grid of magnitudes covering 20 Hz to 5 kHz.

### 3. Peak extraction (`internal/dsp/peaks.go`)
Each frame is divided into **6 frequency bands**:

| Band | Range |
|------|-------|
| VeryLow | 20–300 Hz |
| Low | 300–600 Hz |
| LowMid | 600–1200 Hz |
| Mid | 1200–2400 Hz |
| HighMid | 2400–4000 Hz |
| High | 4000–5000 Hz |

The loudest bin in each band becomes a candidate. Candidates quieter than the average of that frame's candidates are dropped. This dynamic threshold adapts to how loud each moment is, so quiet passages and silence don't produce false peaks.

### 4. Hashing, or the constellation map (`internal/dsp/fingerprint.go`)
Every peak works as an **anchor** and is paired with up to **5 target peaks** that follow it within about 4.6 seconds. Each pair is packed into one 32-bit integer:

```
[ anchor freq bin : 9 bits ][ target freq bin : 9 bits ][ frame delta : 14 bits ]
```

Each hash is stored together with the anchor's frame index (`anchor_time`). A typical 3-minute track produces about 50,000–100,000 hashes. A pair of frequencies plus their time gap stays stable under noise, compression and volume changes, which is what makes the hashes robust.

### 5. Matching (`internal/matcher/matcher.go`)
1. Every hash from the snippet is looked up in the database (`WHERE hash = ANY($1)`), using the index on `fingerprints.hash`.
2. For every hit, the matcher computes `ΔT = db_anchor_time − snippet_anchor_time` and counts how often each `ΔT` occurs, per song.
3. If the snippet really comes from a song, many hashes line up at the **same** `ΔT`, which shows up as a tall spike in the histogram. Random collisions spread out over many different offsets. The height of the tallest spike is the song's **score**, and its `ΔT` gives the position of the snippet in the track.
4. A song only counts as a match if its score is at least `max(20, 0.3% of the snippet's hash count)`. Scores are sorted and the **top 3** are returned.
5. The score of the best match sets the confidence label: **high** at 50 or more, otherwise **medium**.

---

## Database design

PostgreSQL. When the server starts, it creates the schema automatically (`internal/store/migrations.go`). Reference SQL files with comments are in `db/migrations/`.

```
┌──────────────────────┐        ┌───────────────────────────┐
│ songs                │        │ fingerprints              │
├──────────────────────┤        ├───────────────────────────┤
│ id          PK       │◄───────┤ song_id   FK (CASCADE)    │
│ title                │   1:N  │ id        BIGSERIAL PK    │
│ artist               │        │ hash      INTEGER  ◄ index│
│ album                │        │ anchor_time INTEGER       │
│ duration_ms          │        └───────────────────────────┘
│ source               │
│ spotify_id           │        ┌───────────────────────────┐
│ file_hash   UNIQUE   │◄───────┤ song_features (planned)   │
│ created_at           │   1:1  │ song_id   FK UNIQUE       │
│ updated_at           │        │ energy, tempo, key, ...   │
└──────────────────────┘        └───────────────────────────┘
```

### `songs`
One row per uploaded track.
- `file_hash`: SHA-256 of the original file. A `UNIQUE` constraint on it prevents duplicate uploads.
- `source`: where the metadata came from (`manual`, `spotify`, `unknown`).
- `spotify_id`: optional, reserved for Spotify integration.
- `duration_ms`: taken from the decoded audio.

### `fingerprints`
One row per hash, so this table is by far the largest: tens of thousands of rows per song.
- `hash`: the 32-bit packed anchor/target/delta value described above.
- `anchor_time`: the frame index of the anchor peak, in **FFT frame units, not milliseconds**. One frame is 512 / 11025 s, about 46 ms.
- `song_id`: foreign key to `songs` with `ON DELETE CASCADE`, so deleting a song also deletes its fingerprints.
- **`idx_fingerprints_hash` is the critical index.** Every identify request runs through it. `idx_fingerprints_song_id` makes deletes and per-song counts fast.

### `song_features` (planned)
Defined only in `db/migrations/003_create_song_features.sql`. The server does not create it automatically yet. It is meant to hold Spotify audio features (energy, danceability, valence, tempo, key, ...) with a 1:1 relationship to `songs`.

---

## API

All endpoints are under `/api/v1`.

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/upload` | Upload a full song (multipart `file`, optional `title`/`artist`/`album`, max 500 MB) |
| POST | `/api/v1/identify` | Identify an audio snippet (multipart `file`, max 10 MB) |
| GET | `/api/v1/songs?page=&page_size=` | List songs (paginated) |
| GET | `/api/v1/songs/{id}` | Get a single song |
| DELETE | `/api/v1/songs/{id}` | Delete a song and its fingerprints |

Example identify response:

```json
{
  "found": true,
  "confidence": "high",
  "matches": [
    { "rank": 1, "song": { "id": 4, "title": "...", "artist": "..." }, "score": 132, "offset_secs": 74.3 }
  ],
  "processed_in": "412ms"
}
```

When nothing matches, the endpoint still returns `200 OK`, with `found: false` and an empty `matches` array.

---

## Project structure

```
cmd/api/              entry point (main.go) and store→handler adapters
internal/config/      .env loading
internal/api/         router, handlers, request/response DTOs
internal/audio/       FFmpeg decoder + normalizer
internal/dsp/         spectrogram, peak extraction, hashing
internal/matcher/     time-offset histogram scoring
internal/store/       Postgres stores + embedded migrations
db/migrations/        reference SQL schema
frontend/             Vue 3 app (views, Pinia stores, recorder composable)
```

---

## Running the project

### Requirements
- Go 1.24+
- Node 20+
- PostgreSQL (running, with a database and user created)
- FFmpeg installed and on `PATH`

### Setup
1. Copy `.env.example` to `.env` in the project root and fill in the `DB_*` values.
2. Start the backend from the project root. It listens on `:8080`, or on `APP_PORT` if set:
   ```bash
   go run ./cmd/api/        # or: make run
   ```
3. Start the frontend:
   ```bash
   cd frontend
   npm install
   npm run dev              # https://localhost:5173
   ```
   Vite forwards `/api/*` requests to `http://localhost:8080`, so the frontend works from any device on your network.

### Build and test

```bash
# Backend
go build ./...          # or: make build
go test ./...           # or: make test
make vet

# Frontend
cd frontend
npm run build
npm run test:unit
npm run lint
```

---

## Dependencies

### PostgreSQL driver for Go
```bash
go get github.com/lib/pq
```

### Chi router: lightweight, idiomatic HTTP router
```bash
go get github.com/go-chi/chi/v5
```

### Chi middleware package
```bash
go get github.com/go-chi/chi/v5/middleware
```

### CORS middleware
```bash
go get github.com/go-chi/cors
```

### Environment variable loading from .env file
```bash
go get github.com/joho/godotenv
```

### Gonum DSP for FFT
```bash
go get gonum.org/v1/gonum/dsp/fourier
```

### Gonum math utilities
```bash
go get gonum.org/v1/gonum/floats
```

### Audio metadata tag reader (ID3 / FLAC / M4A / OGG)
```bash
go get github.com/dhowden/tag
```

### UUID generation (for any future need)
```bash
go get github.com/google/uuid
```

### Tidy up go.mod and go.sum
```bash
go mod tidy
```
