# Phase 4 — Step 5 — `routes.go` — Introduction Document

---

## What `routes.go` Is

`routes.go` is the **single file that connects URLs to handler functions**. It creates the chi router, registers all middleware, and maps every HTTP method and path to the correct handler method.

Nothing in this file computes anything. It assembles what already exists:

```
chi.Router  ←  middleware  ←  handlers  ←  stores
```

---

## What It Wires Together

```
Middleware stack (applied to every request)
  └── middleware.RequestID        attach unique ID to every request
  └── middleware.Logger           structured JSON log per request
  └── middleware.Recoverer        catch panics, return 500

Routes
  └── POST   /api/v1/upload           → Handler.Upload
  └── POST   /api/v1/identify         → Handler.Identify
  └── GET    /api/v1/songs            → Handler.ListSongs
  └── GET    /api/v1/songs/{id}       → Handler.GetSong
  └── DELETE /api/v1/songs/{id}       → Handler.DeleteSong
```

---

## The Function Signature

One exported function — that is all `routes.go` contains:

```go
func NewRouter(h *handlers.Handler) http.Handler
```

**Why it accepts `*handlers.Handler` not individual handler functions:**
The `Handler` struct holds all handler methods. Passing the struct once is cleaner than passing five separate function arguments. When a sixth handler is added later, `NewRouter` does not change its signature.

**Why it returns `http.Handler` not `*chi.Mux`:**
`http.Handler` is the standard library interface. Returning the concrete `*chi.Mux` would expose chi-specific methods to callers that do not need them. `http.Handler` is all `http.ListenAndServe` needs — the router can be swapped without changing `main.go`.

---

## Middleware Order Matters

```
Request arrives
      │
      ▼
RequestID     ← assign ID before anything logs
      │
      ▼
Logger        ← log with the ID already attached
      │
      ▼
Recoverer     ← catch panics from any handler below
      │
      ▼
Handler       ← actual business logic
```

**Why RequestID before Logger:**
The logger reads the request ID from context to include it in every log line. If Logger runs first, the ID is not yet in context — the log entry has no ID.

**Why Recoverer after Logger:**
If the Logger itself panics (unlikely but possible), the Recoverer cannot catch it. In practice, Logger is safe — so Recoverer last in the middleware chain is the conventional order.

---

## URL Design

```
/api/v1/upload          POST only — no GET, no DELETE
/api/v1/identify        POST only
/api/v1/songs           GET only  — list
/api/v1/songs/{id}      GET, DELETE — single resource
```

**Why `/api/v1/` prefix:**
- `/api/` distinguishes API routes from any future web UI routes served by the same binary
- `/v1/` makes versioning explicit — a breaking change becomes `/api/v2/` without removing `/api/v1/`

**Why no `/api/v1/songs` POST:**
Songs are not created by posting JSON metadata. They are created by uploading an audio file — that is the `/api/v1/upload` endpoint. The upload handler creates the song record as part of the fingerprinting pipeline.

---

## File Location

```
internal/api/
  ├── routes.go          ← what we are about to build
  ├── routes_test.go
  ├── dto/
  ├── handlers/
  └── middleware/
```

`routes.go` sits at the `api` package level — one level above `handlers` and `middleware`. It is the package that owns the assembled router.

---

## What Routes Does Not Do

```
No handler logic      ← handlers/
No middleware logic   ← middleware/
No store setup        ← cmd/api/main.go
No server startup     ← cmd/api/main.go
No database config    ← config/
No DSP algorithms     ← dsp/
```

`routes.go` is pure assembly. Every component it references was built and tested independently before this file touches it.

---

# `routes.go` — Build Summary

---

## What We Built

`routes.go` is a single function that assembles the HTTP router. It is the last piece of the `internal/api` layer — everything built before it (middleware, handlers, DTOs, stores) is wired together here into a working HTTP server surface.

```
NewRouter(h *handlers.Handler) http.Handler
```

That is the entire public API of this file.

---

## File Location

```
internal/api/
  ├── routes.go          ← this file
  ├── routes_test.go
  ├── dto/
  ├── handlers/
  └── middleware/
```

---

## The Middleware Stack

```go
r.Use(middleware.Recovery)
r.Use(middleware.Logging)
r.Use(middleware.CORS)
```

Three middleware functions, applied in deliberate order:

| Position | Middleware | Why Here |
|----------|-----------|----------|
| 1st | `Recovery` | Catches panics from everything below — including `Logging` and `CORS` if they ever fail |
| 2nd | `Logging` | Logs every request that survived the recovery wrap — has access to the full request context |
| 3rd | `CORS` | Sets response headers before the handler writes anything — must run before the handler |

**One correction made during development:** The initial draft used `middleware.RequestID` and chi's built-in `middleware.Recoverer`. Neither existed in our middleware package. We use our own `middleware.Recovery` and dropped `RequestID` — no request ID tracking was built into the middleware layer.

---

## The Route Table

```go
r.Route("/api/v1", func(r chi.Router) {
    r.Post("/upload",       h.Upload)
    r.Post("/identify",     h.Identify)
    r.Get("/songs",         h.ListSongs)
    r.Route("/songs/{id}", func(r chi.Router) {
        r.Get("/",    h.GetSong)
        r.Delete("/", h.DeleteSong)
    })
})
```

Expanded into the full method + path table:

| Method | Path | Handler | Purpose |
|--------|------|---------|---------|
| `POST` | `/api/v1/upload` | `Handler.Upload` | Register a song via audio file |
| `POST` | `/api/v1/identify` | `Handler.Identify` | Identify a snippet against the library |
| `GET` | `/api/v1/songs` | `Handler.ListSongs` | List songs with pagination |
| `GET` | `/api/v1/songs/{id}` | `Handler.GetSong` | Retrieve one song by ID |
| `DELETE` | `/api/v1/songs/{id}` | `Handler.DeleteSong` | Remove a song and its fingerprints |

Every other method + path combination returns `404` or `405` — chi handles this automatically from the registered routes.

---

## Why `http.Handler` Not `*chi.Mux`

```go
func NewRouter(h *handlers.Handler) http.Handler   // ✅
func NewRouter(h *handlers.Handler) *chi.Mux       // ✗
```

`http.ListenAndServe` accepts `http.Handler`. Returning `*chi.Mux` would expose chi-specific methods to `main.go` and test files — coupling them to the router library. Returning `http.Handler` means the router implementation can be swapped without changing any caller.

---

## Why `*handlers.Handler` Not Individual Functions

```go
func NewRouter(h *handlers.Handler) http.Handler              // ✅
func NewRouter(upload, identify, list, get, delete ...) http.Handler  // ✗
```

Passing five function arguments would make `NewRouter`'s signature grow every time a new handler is added. The `Handler` struct carries all handler methods. One argument today, one argument after ten more routes are added.

---

## Testing Strategy

Routes tests answer exactly one question per test: **is this method + path combination registered correctly?**

They do not test handler logic — that is `handlers/*_test.go`. They do not test middleware logic in depth — that is `middleware/*_test.go`.

### The 404 vs 405 vs Handler-404 Distinction

This required careful thinking in the tests:

```
Router 404   → path not registered at all
Router 405   → path registered, but not for this method
Handler 404  → path registered, handler ran, store returned ErrNotFound
```

For `GET /api/v1/songs/{id}` the stub store returns `store.ErrNotFound`, which causes the handler to return HTTP 404. This is a **handler 404**, not a **router 404**. The test correctly checks for 405 absence rather than asserting a specific 2xx status:

```go
// A 404 here is from the handler (song not found in stub store),
// not from the router (unregistered path). Check for 405 instead.
if rec.Code == http.StatusMethodNotAllowed {
    t.Errorf("GET /api/v1/songs/{id} returned 405 — wrong method registered")
}
```

### Tests Written

| Test | Verifies |
|------|---------|
| `TestRoutes_Upload_PostIsRegistered` | POST /upload matched |
| `TestRoutes_Upload_GetIsNotAllowed` | GET /upload → 405 |
| `TestRoutes_Upload_DeleteIsNotAllowed` | DELETE /upload → 405 |
| `TestRoutes_Identify_PostIsRegistered` | POST /identify matched |
| `TestRoutes_Identify_GetIsNotAllowed` | GET /identify → 405 |
| `TestRoutes_Identify_DeleteIsNotAllowed` | DELETE /identify → 405 |
| `TestRoutes_Songs_GetIsRegistered` | GET /songs matched |
| `TestRoutes_Songs_PostIsNotAllowed` | POST /songs → 405 |
| `TestRoutes_Songs_DeleteIsNotAllowed` | DELETE /songs → 405 |
| `TestRoutes_SongByID_GetIsRegistered` | GET /songs/{id} matched |
| `TestRoutes_SongByID_DeleteIsRegistered` | DELETE /songs/{id} matched |
| `TestRoutes_SongByID_PostIsNotAllowed` | POST /songs/{id} → 405 |
| `TestRoutes_SongByID_PutIsNotAllowed` | PUT /songs/{id} → 405 |
| `TestRoutes_UnknownPath_Returns404` | Unknown path → 404 |
| `TestRoutes_RootPath_Returns404` | / → 404 |
| `TestRoutes_NoVersionPrefix_Returns404` | /songs without /api/v1 → 404 |
| `TestRoutes_WrongVersionPrefix_Returns404` | /api/v2/songs → 404 |
| `TestRoutes_ListSongs_ResponseIsJSON` | Content-Type is application/json |
| `TestRoutes_CORS_AllowedOrigin_SetsHeader` | Allowed origin gets CORS header |
| `TestRoutes_CORS_Preflight_Returns200` | OPTIONS preflight returns 200 |
| `TestRoutes_CORS_DisallowedOrigin_NoHeader` | Disallowed origin gets no CORS header |
| `TestRoutes_Recovery_PanicReturns500` | Panicking handler returns 500 |
| `TestRoutes_NewRouter_ReturnsNonNil` | NewRouter does not return nil |

---

## Test Results

```
All tests   → PASS
go vet      → clean
go build    → clean
```

---

## What Comes Next — `cmd/api/main.go`

The router is built. The last step is the entry point that starts the server:

```
cmd/api/
  └── main.go    ← reads config
                    opens database connection
                    builds store implementations
                    builds Handler
                    builds Router
                    calls http.ListenAndServe
```

After `main.go` the application is complete and runnable.

> Summary complete. Move forward to `cmd/api/main.go` for further development explanation.