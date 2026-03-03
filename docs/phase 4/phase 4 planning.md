# Phase 4 — Planning & Component Order

Before we touch a single file, let us establish exactly what Phase 4 contains, what order we build it in, and why.

---

## What Phase 4 Is

Phase 4 is the **glue layer** that connects the DSP engine to the outside world. It has three distinct responsibilities:

```
DSP Engine (Phase 3)
      │
      ▼
┌─────────────────────────────────────────┐
│              Phase 4                    │
│                                         │
│  models/     → shared data structures  │
│  store/      → database read/write     │
│  matcher/    → coherence scoring       │
│  api/        → HTTP server & handlers  │
└─────────────────────────────────────────┘
      │
      ▼
Frontend (Phase 5)
```

---

## Build Order and Why

The order matters. Each component depends on the one before it.

```
Step 1 → models/          No dependencies — pure data structs
                          Everything else imports these

Step 2 → store/           Depends on models
                          song_store.go first, then fingerprint_store.go

Step 3 → matcher/         Depends on models and store
                          Coherence scoring and Top 3 selection

Step 4 → api/middleware   Depends on nothing — pure HTTP concerns
                          logging, recovery, cors

Step 5 → api/dto          Depends on models
                          Request and response shapes

Step 6 → api/handlers     Depends on store, matcher, dto, audio, dsp
                          The final assembly point

Step 7 → api/routes       Depends on handlers and middleware
                          Wires everything together

Step 8 → cmd/api/main.go  Depends on everything
                          Starts the server
```

---

## Component Breakdown

### Step 1 — `internal/models/`

Three files. Pure Go structs, no logic, no database calls, no imports from our own packages.

```
models/song.go
  └── Song struct — mirrors the songs table exactly

models/fingerprint.go
  └── Fingerprint struct — mirrors the fingerprints table exactly

models/song_feature.go
  └── SongFeature struct — mirrors the song_features table exactly
```

These are the shared language of the application. Every other layer speaks in terms of these structs.

---

### Step 2 — `internal/store/`

Two files. All database read and write operations live here. Nothing outside this package touches SQL directly.

```
store/song_store.go
  └── Insert(song)
  └── GetByID(id)
  └── GetByFileHash(hash)
  └── List(page, pageSize)
  └── Delete(id)

store/fingerprint_store.go
  └── InsertBatch(hashes)       ← bulk insert, performance critical
  └── QueryByHashes(hashes)     ← the matching query, index critical
```

---

### Step 3 — `internal/matcher/`

One file. Receives raw query results from `fingerprint_store.go` and produces a ranked Top 3 result.

```
matcher/matcher.go
  └── Match(queryHashes, dbResults) → []MatchResult
      - builds in-memory HashMap
      - calculates coherence scores
      - returns Top 3 sorted by score
```

---

### Step 4 — `internal/api/middleware/`

Three files. Pure HTTP middleware — no business logic.

```
middleware/logging.go    ← logs every request and response time
middleware/recovery.go   ← catches panics, returns 500
middleware/cors.go       ← sets CORS headers for browser requests
```

---

### Step 5 — `internal/api/dto/`

Two files. Defines the exact JSON shapes the API accepts and returns.

```
dto/request.go
  └── UploadRequest
  └── IdentifyRequest

dto/response.go
  └── SongResponse
  └── MatchResponse
  └── ErrorResponse
```

---

### Step 6 — `internal/api/handlers/`

Three files. Each handler is responsible for one group of endpoints.

```
handlers/upload.go
  └── POST /api/v1/upload
      - accept multipart file
      - decode + normalize + fingerprint
      - store song + hashes in DB

handlers/identify.go
  └── POST /api/v1/identify
      - accept multipart audio snippet
      - decode + normalize + fingerprint
      - query DB + run matcher
      - return Top 3

handlers/songs.go
  └── GET    /api/v1/songs
  └── GET    /api/v1/songs/:id
  └── DELETE /api/v1/songs/:id
```

---

### Step 7 — `internal/api/routes.go`

One file. Wires all handlers and middleware together onto the chi router.

```
routes.go
  └── NewRouter(stores, matcher, cfg) → http.Handler
      - mounts all middleware
      - registers all routes
      - returns the complete handler tree
```

---

### Step 8 — `cmd/api/main.go`

The entry point. Opens the database connection, initialises all stores, builds the router, starts the HTTP server.

```
main.go
  └── Load config
  └── Connect to PostgreSQL
  └── Initialise stores
  └── Build router
  └── ListenAndServe
```

---

## What Gets Tested at Each Step

| Step | Test Type |
|------|-----------|
| models | Struct field types, JSON tags, zero values |
| store | Real PostgreSQL queries using SQLite for unit tests |
| matcher | Pure logic — coherence scoring, ranking, edge cases |
| middleware | HTTP handler chain behaviour |
| dto | JSON marshal and unmarshal correctness |
| handlers | HTTP integration tests — request in, response out |
| routes | Route registration, method not allowed, 404 |
| main | Compile and startup only |

---

## Database Connection Strategy for Tests

The store tests need a database. We use two approaches:

```
Unit tests    → SQLite in-memory database
              → Fast, no PostgreSQL required
              → Tests run anywhere including CI

Integration   → Real PostgreSQL (local djfingerprint DB)
tests           → Uses a separate test schema
              → Cleaned up after each test run
```

This matches the strategy established in Phase 1.

---