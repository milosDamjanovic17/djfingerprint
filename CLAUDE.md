# CLAUDE.md — 12-rule template

These rules apply to every task in this project unless explicitly overridden.
Bias: caution over speed on non-trivial work.

## Rule 1 — Think Before Coding
State assumptions explicitly. Ask rather than guess.
Push back when a simpler approach exists. Stop when confused.

## Rule 2 — Simplicity First
Minimum code that solves the problem. Nothing speculative.
No abstractions for single-use code.

## Rule 3 — Surgical Changes
Touch only what you must. Don't improve adjacent code.
Match existing style. Don't refactor what isn't broken.

## Rule 4 — Goal-Driven Execution
Define success criteria. Loop until verified.
Strong success criteria let Claude loop independently.

## Rule 5 — Use the model only for judgment calls
Use for: classification, drafting, summarization, extraction.
Do NOT use for: routing, retries, deterministic transforms.
If code can answer, code answers.

## Rule 6 — Token budgets are not advisory
Per-task: 4,000 tokens. Per-session: 30,000 tokens.
If approaching budget, summarize and start fresh.
Surface the breach. Do not silently overrun.

## Rule 7 — Surface conflicts, don't average them
If two patterns contradict, pick one (more recent / more tested).
Explain why. Flag the other for cleanup.

## Rule 8 — Read before you write
Before adding code, read exports, immediate callers, shared utilities.
If unsure why existing code is structured a certain way, ask.

## Rule 9 — Tests verify intent, not just behavior
Tests must encode WHY behavior matters, not just WHAT it does.
A test that can't fail when business logic changes is wrong.

## Rule 10 — Checkpoint after every significant step
Summarize what was done, what's verified, what's left.
Don't continue from a state you can't describe back.

## Rule 11 — Match the codebase's conventions, even if you disagree
Conformance > taste inside the codebase.
If you think a convention is harmful, surface it. Don't fork silently.

## Rule 12 — Fail loud
"Completed" is wrong if anything was skipped silently.
"Tests pass" is wrong if any were skipped.
Default to surfacing uncertainty, not hiding it.

# DJ Fingerprint — Project Reference

## Overview

Shazam-like music fingerprinting web application. Users upload full songs to build a library, then identify unknown audio by recording a snippet (microphone or browser audio capture). Backend in Go, frontend in Vue 3 + TypeScript.

## Architecture

### Backend (Go)

- **Entry point**: `cmd/api/main.go` — loads config, opens Postgres, runs migrations, builds stores/adapters, starts HTTP server with graceful shutdown.
- **Config**: `internal/config/config.go` — loaded from `.env` at project root. Key vars: `APP_PORT`, `DB_*`, audio/fingerprint params.
- **Router**: `internal/api/routes.go` — chi router, all endpoints under `/api/v1`.
- **Handlers**: `internal/api/handlers/` — `upload.go`, `identify.go`, `songs.go`, `handler.go`.
- **DTOs**: `internal/api/dto/` — request parsing (`request.go`) and response shapes (`response.go`).
- **Audio pipeline**: `internal/audio/decoder.go` (FFmpeg decode to 11025 Hz mono PCM), `normalizer.go` (peak normalization).
- **DSP/Fingerprinting**: `internal/dsp/` — `spectrogram.go` (FFT + Hamming window), `peaks.go` (6-band peak extraction), `fingerprint.go` (constellation map hashing with fan-out=5).
- **Matcher**: `internal/matcher/matcher.go` — time-offset histogram coherence scoring. MinScore=5, TopN=3.
- **Database**: PostgreSQL. Stores in `internal/store/` — `song_store.go`, `fingerprint_store.go`. Migrations in `db/migrations/` and embedded in `internal/store/migrations.go`.
- **Adapters**: `cmd/api/adapters.go` — translate between store implementations and handler interfaces.

### Frontend (Vue 3 + TypeScript)

- **Build**: Vite + Tailwind CSS 4 + Pinia state management.
- **API client**: `frontend/src/api/client.ts` — Axios, uses relative baseURL (empty string) so Vite proxy routes `/api/*` to Go backend.
- **API layer**: `frontend/src/api/` — `songs.ts`, `identify.ts`.
- **Stores**: `frontend/src/stores/` — `upload.ts`, `identify.ts`, `library.ts`.
- **Views**: `UploadView.vue`, `IdentifyView.vue`, `LibraryView.vue`.
- **Composables**: `useRecorder.ts` — supports `'microphone'` and `'browser'` audio sources.
- **Components**: `UploadZone.vue`, `NavBar.vue`.

### API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/upload` | Upload full song (multipart, max 500MB) |
| POST | `/api/v1/identify` | Identify audio snippet (multipart, max 10MB) |
| GET | `/api/v1/songs` | List songs (paginated: `page`, `pageSize`) |
| GET | `/api/v1/songs/{id}` | Get single song |
| DELETE | `/api/v1/songs/{id}` | Delete song + fingerprints |

## Running the Project

- **Backend**: `go run ./cmd/api/` from project root. Listens on `:8080` (configurable via `APP_PORT` in `.env`).
- **Frontend**: `cd frontend && npm run dev`. Vite serves on `https://localhost:5173` with HTTPS (self-signed cert via `@vitejs/plugin-basic-ssl`).
- **Requirements**: PostgreSQL running, FFmpeg installed and on PATH, Go 1.24+, Node 20+.
- **LAN access**: Vite is configured with `host: true` and HTTPS. Open `https://<machine-ip>:5173` on any device on the same network. Accept the self-signed cert warning.
- **HTTPS is required**: `getUserMedia` (microphone) and `getDisplayMedia` (browser audio) only work over HTTPS on non-localhost origins. The `@vitejs/plugin-basic-ssl` plugin handles this for dev.

## Key Decisions & Fixes Applied

### Filename-based metadata fallback (upload handler)
- **Problem**: DJ tracks (Beatport, etc.) often have no embedded ID3 tags. Uploading them resulted in "Unknown Title / Unknown Artist" in the library.
- **Fix**: Added `parseFilename()` in `internal/api/handlers/upload.go` that parses the `"Artist - Title.ext"` convention as a last-resort fallback.
- **Priority chain**: form fields > embedded ID3 tags > filename parsing.

### HTTPS for dev server (Vite)
- **Problem**: Phone microphone silently denied — `getUserMedia` requires secure context. Browser audio (`getDisplayMedia`) also blocked on `http://`.
- **Fix**: Added `@vitejs/plugin-basic-ssl` to `vite.config.ts`. Vite now serves over HTTPS with an auto-generated self-signed certificate.
- **Phone usage**: User must tap "Advanced → Proceed" past the cert warning on first visit.

### Browser audio capture (identify feature)
- **Added**: `useRecorder.ts` now accepts `source: 'microphone' | 'browser'`.
- **Browser mode** uses `getDisplayMedia({ video: true, audio: true })`, discards video tracks immediately, records only audio.
- **Chrome/Edge only** — Firefox and Safari have limited `getDisplayMedia` audio support.
- **UI**: Toggle switch in `IdentifyView.vue` between "Microphone" and "Browser Audio" with a hint for Chrome users to tick "Share tab audio".

### Vite proxy for API calls
- **Problem**: Frontend used hardcoded `VITE_API_URL=http://localhost:8080` — breaks when accessed from phone (localhost doesn't resolve).
- **Fix**: `client.ts` uses empty `baseURL` (relative URLs). Vite config has `proxy: { '/api': { target: 'http://localhost:8080' } }`. All API requests from any device route through Vite to the Go backend.

## Known Issues / Warnings

### API response mismatch (identify endpoint)
- **Frontend** (`frontend/src/api/identify.ts`): `IdentifyResponse` expects `{ match: ... | null }`.
- **Backend** (`internal/api/dto/response.go`): returns `{ found, confidence, matches[], processedIn }`.
- **Impact**: Identify results may not render correctly. The frontend interface needs to be updated to match the backend DTO.

### migrations.go anchor_time type
- `internal/store/migrations.go` line 52 defines `anchor_time REAL` but the Go struct uses `int` and the standalone SQL migration (`db/migrations/002_create_fingerprints.sql`) correctly uses `INTEGER`.
- If the database was created via the standalone SQL files, this is fine. If created via the embedded Go migration, there's a type mismatch.

### Duplicate code in upload handler
- `internal/api/handlers/upload.go` lines ~122-128 contain a duplicate hash generation check that's unreachable. Harmless but should be cleaned up.

### False positive elimination (MinScore raised + ratio check)
- **Problem**: App never returned "No match" — even songs not in the database always matched something. Root cause: `MinScore = 5` was too low. With 6 frequency bands and concentrated bin values, electronic/pop music shares common frequency patterns (kicks at 40-200 Hz, hi-hats at 3-5 kHz). This shrinks the effective hash space and causes enough random collisions to exceed a threshold of 5.
- **Fix** (in `internal/matcher/matcher.go`):
  - `MinScore` raised from **5 → 20**. A genuine 10-second match produces 50-200+ coherent hashes; random collisions between unrelated songs rarely exceed 10-15.
  - Added **`MinScoreRatio = 0.003`** (0.3% of total query hashes). The effective threshold is `max(MinScore, totalQueryHashes * MinScoreRatio)`. This prevents weak matches when hash count is very high.
- **Impact**: Songs not in the library now correctly return "No match". Genuine matches (which score 50+) are unaffected.
- **Tests**: Updated all matcher tests to use scores above the new thresholds. Added `TestFilterByMinScore_RatioThreshold_Applies` to verify the ratio check.

## Build & Test Commands

```bash
# Backend
go build ./...                    # compile all
go test ./...                     # run all tests
go run ./cmd/api/                 # start server

# Frontend
cd frontend
npm install                       # install deps
npm run dev                       # dev server (HTTPS, LAN-accessible)
npm run build                     # production build
npm run test:unit                 # run vitest
npm run lint                      # eslint + oxlint
```
