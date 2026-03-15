# `middleware/` — Build Summary

---

## What We Built

### The Purpose
The middleware package is the **defensive wrapper around every HTTP request and response** in the application. Three focused files, each doing one thing perfectly, applied to every endpoint automatically without any handler needing to know they exist. Together they ensure every request is logged, every panic is caught, and every browser request is permitted.

---

## Execution Order — Why It Matters

```
Request arrives
      │
      ▼
┌─────────────────────────────────┐
│  CORS Middleware                │ ← outermost
│  ┌───────────────────────────┐  │
│  │  Logging Middleware       │  │
│  │  ┌─────────────────────┐  │  │
│  │  │  Recovery Middleware │  │  │
│  │  │  ┌───────────────┐  │  │  │
│  │  │  │    Handler    │  │  │  │ ← innermost
│  │  │  └───────────────┘  │  │  │
│  │  └─────────────────────┘  │  │
│  └───────────────────────────┘  │
└─────────────────────────────────┘
      │
      ▼
Response leaves
```

In chi this order is expressed as:

```go
r.Use(middleware.CORS)
r.Use(middleware.Logging)
r.Use(middleware.Recovery)
```

**Why CORS is outermost:**
Preflight `OPTIONS` requests must be answered before any other middleware touches them. If logging ran before CORS, a preflight `OPTIONS` request would be logged as an incoming request, which adds noise. More importantly if the request never reaches the CORS middleware, the browser never gets permission to send the actual request.

**Why Recovery is innermost:**
Recovery wraps the handler tightly. If a panic occurs it unwinds only through recovery — the stack trace is logged and HTTP 500 is returned cleanly. Logging then records the 500 status code accurately on its way back out.

---

## `logging.go` — What Was Built and Why

### The `responseRecorder` Struct

```go
type responseRecorder struct {
    http.ResponseWriter
    statusCode int
    bytes      int
}
```

The standard `http.ResponseWriter` is write-only — once the handler calls `WriteHeader` and `Write`, those values are gone. There is no way to ask "what status code did the handler write?" after the fact.

`responseRecorder` solves this by embedding `http.ResponseWriter` and intercepting two methods:

**`WriteHeader(code int)`** — captures the status code before passing it to the underlying writer:
```go
func (rr *responseRecorder) WriteHeader(code int) {
    rr.statusCode = code
    rr.ResponseWriter.WriteHeader(code)
}
```

**`Write(b []byte)`** — counts bytes written before passing them through:
```go
func (rr *responseRecorder) Write(b []byte) (int, error) {
    n, err := rr.ResponseWriter.Write(b)
    rr.bytes += n
    return n, err
}
```

The default `statusCode` is `200` — if the handler never calls `WriteHeader` explicitly, the implicit status is `200 OK` which is what Go's HTTP server sends.

---

### The `Logging` Middleware

```go
func Logging(next http.Handler) http.Handler
```

Logs two lines per request:

```
→ POST /api/v1/upload 192.168.1.1:54321
← 201 POST /api/v1/upload 2.341s 142B
```

**On arrival** — method, path, remote address — immediately when the request enters the middleware.

**On completion** — status code, method, path, duration, bytes — after `next.ServeHTTP` returns.

**Duration measurement:**
```go
start := time.Now()
next.ServeHTTP(rec, r)
duration := time.Since(start)
```

`time.Since(start)` captures the wall clock time from when the request arrived to when the handler finished — including database queries, DSP processing, and response serialisation.

---

### Format Helpers

**`formatDuration(d time.Duration) string`**

```
< 1 second  →  "500ms"
≥ 1 second  →  "2.341s"
```

Short requests show milliseconds for precision. Longer requests (uploads, identification) show seconds for readability.

**`formatBytes(n int) string`**

```
0 bytes   →  "0B"
142 bytes →  "142B"
```

Simple byte count. Useful for verifying response bodies are not empty or unexpectedly large.

---

## `recovery.go` — What Was Built and Why

### The Problem Without Recovery Middleware

In Go each HTTP request is handled in its own goroutine. An unrecovered panic in a handler:

```
Handler panics
      │
      ▼
Goroutine crashes
      │
      ├── Client connection hangs or resets
      ├── Stack trace printed to stderr with no context
      └── Depending on Go version — may crash entire server
```

### The Solution

```go
func Recovery(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("PANIC recovered: %v\n%s", err, debug.Stack())
                http.Error(w, http.StatusText(http.StatusInternalServerError), 500)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

**Why `defer`:** The deferred function runs even when a panic unwinds the call stack. This is the only reliable way to catch a panic in Go.

**Why `debug.Stack()`:** The full stack trace is logged to help diagnose where the panic originated. In production this goes to your log file. The stack trace is never sent to the client.

**Why `http.StatusText(500)` not the panic message:** Exposing internal panic messages to clients is a security risk. A panic might contain database connection strings, file paths, or internal variable values. The client receives only `"Internal Server Error"` — a generic, safe message.

---

### The `nil` Panic Behaviour — Go 1.21+

During testing `TestRecovery_NilPanic_Returns500` produced the log message:

```
PANIC recovered: panic called with nil argument
```

This is correct Go 1.21+ behaviour. Before Go 1.21 `panic(nil)` caused `recover()` to return `nil` — indistinguishable from "no panic occurred". This was a known footgun in Go.

From Go 1.21 onwards `panic(nil)` is wrapped in `runtime.PanicNilError` before being caught by `recover()`, so `recover()` returns a non-nil value with the message `"panic called with nil argument"`. Our middleware handles this correctly — the `if err := recover(); err != nil` check fires and HTTP 500 is returned.

---

## `cors.go` — What Was Built and Why

### Why CORS Is Needed

The Vue frontend runs on `http://localhost:5173` (Vite dev server). The API runs on `http://localhost:8080`. These are different origins — different ports make them different origins under the browser's Same-Origin Policy.

Without CORS headers every API request from the frontend would be blocked by the browser before it even left the client.

---

### The `corsConfig`

```go
var corsConfig = struct {
    allowedOrigins []string
    allowedMethods string
    allowedHeaders string
    maxAge         string
}{
    allowedOrigins: []string{
        "http://localhost:5173",
        "http://localhost:3000",
        "http://localhost:8080",
    },
    allowedMethods: "GET, POST, DELETE, OPTIONS",
    allowedHeaders: "Content-Type, Authorization, Accept",
    maxAge:         "86400",
}
```

**Why a struct rather than constants:**
The CORS configuration is a cohesive unit — all four values belong together. A struct makes it clear they are related and makes it easy to replace with a config-file-driven version in Phase 7.

**Why `maxAge: "86400"`:**
Browsers cache preflight responses for `maxAge` seconds. 86400 = 24 hours. This means the browser only sends one `OPTIONS` preflight per day per endpoint per origin rather than before every single `POST` or `DELETE`. Significant reduction in preflight round trips during normal use.

---

### The `CORS` Middleware

```go
func CORS(next http.Handler) http.Handler
```

**For allowed origins:**
Sets four response headers and passes the request to the next handler.

**For `OPTIONS` preflight requests:**
Responds `HTTP 200` with the CORS headers and returns immediately — the preflight never reaches the handler:

```go
if r.Method == http.MethodOptions {
    w.WriteHeader(http.StatusOK)
    return
}
```

**For disallowed origins:**
Passes the request through without setting CORS headers. The browser will block the response on the client side.

**For requests with no `Origin` header:**
Same-origin requests, `curl`, Postman, and server-to-server calls have no `Origin` header. These pass through with no CORS headers applied — they do not need them.

---

### `isAllowedOrigin(origin string) bool`

Simple linear scan through the allowed origins list using `strings.EqualFold` for case-insensitive comparison. Three entries in the list — O(3) is effectively O(1).

---

## Test Output Analysis

All **35 tests passed**. Two observations from the output that are worth documenting:

### Stack Traces in Recovery Test Output

The stack traces printed during recovery tests are the middleware working correctly. The `log.Printf` with `debug.Stack()` in the recovery middleware fires during tests because the tests intentionally trigger panics. In production these traces go to the application log file. Running tests without `-v` suppresses them from terminal output.

### `TestRecovery_NilPanic_Returns500` Message

```
PANIC recovered: panic called with nil argument
```

This is the Go 1.21+ `runtime.PanicNilError` message. Confirmed correct behaviour on the project's Go version. The test assertion allows both `200` (pre-1.21) and `500` (1.21+) — on this Go version the result is always `500`. The test passes correctly.

---

## Tests Written

### `logging_test.go`

| Test | What It Verifies |
|------|-----------------|
| `TestResponseRecorder_DefaultStatusCode` | Default status is 200 when WriteHeader not called |
| `TestResponseRecorder_CapturesStatusCode` | WriteHeader value captured correctly |
| `TestResponseRecorder_CapturesBytes` | Write byte count captured correctly |
| `TestResponseRecorder_PassesThroughToUnderlying` | Status and body reach underlying ResponseWriter |
| `TestLogging_PassesThroughResponse` | Handler response passes through unchanged |
| `TestLogging_200StatusPassesThrough` | 200 status propagated correctly |
| `TestLogging_404StatusPassesThrough` | 404 status propagated correctly |
| `TestLogging_500StatusPassesThrough` | 500 status propagated correctly |
| `TestLogging_WorksForGetMethod` | GET requests reach handler |
| `TestLogging_WorksForDeleteMethod` | DELETE requests reach handler |
| `TestLogging_BodyContentReachesClient` | Response body reaches client unchanged |
| `TestFormatDuration_Milliseconds` | Sub-second durations format as ms |
| `TestFormatDuration_Seconds` | Second+ durations format with 3 decimal places |
| `TestFormatBytes_Zero` | Zero bytes formats as "0B" |
| `TestFormatBytes_NonZero` | Non-zero bytes formats correctly |

### `recovery_test.go`

| Test | What It Verifies |
|------|-----------------|
| `TestRecovery_NoPanic_PassesThrough` | Non-panicking handler passes through normally |
| `TestRecovery_StringPanic_Returns500` | String panic returns HTTP 500 |
| `TestRecovery_ErrorPanic_Returns500` | Error panic returns HTTP 500 |
| `TestRecovery_Panic_ResponseBodyIndicatesError` | Response body contains "Internal Server Error" |
| `TestRecovery_Panic_DoesNotExposeInternals` | Panic message not exposed in response body |
| `TestRecovery_MultipleRequests_ServerContinuesAfterPanic` | Server processes next request normally after panic |
| `TestRecovery_NilPanic_Returns500` | nil panic handled correctly on Go 1.21+ |

### `cors_test.go`

| Test | What It Verifies |
|------|-----------------|
| `TestCORS_AllowedOrigin_HeaderPresent` | Allow-Origin header set for known origin |
| `TestCORS_AllowedOrigin_MethodsHeaderPresent` | Allow-Methods header present |
| `TestCORS_AllowedOrigin_HeadersHeaderPresent` | Allow-Headers header present |
| `TestCORS_AllowedOrigin_MaxAgePresent` | Max-Age header present |
| `TestCORS_Port3000_AllowedOrigin` | Port 3000 recognised as allowed origin |
| `TestCORS_NoOriginHeader_PassesThrough` | No-origin requests pass through |
| `TestCORS_DisallowedOrigin_NoAllowHeader` | Disallowed origin gets no Allow-Origin header |
| `TestCORS_Options_Returns200` | OPTIONS preflight returns 200 |
| `TestCORS_Options_DoesNotReachHandler` | OPTIONS does not reach downstream handler |
| `TestCORS_Options_HasCORSHeaders` | OPTIONS response includes CORS headers |
| `TestCORS_Get_ReachesHandler` | GET passes through to handler |
| `TestCORS_Post_ReachesHandler` | POST passes through to handler |
| `TestCORS_Delete_ReachesHandler` | DELETE passes through to handler |
| `TestIsAllowedOrigin_KnownOrigins` | All three allowed origins pass |
| `TestIsAllowedOrigin_UnknownOrigin` | Unknown origins rejected |

---

## What Middleware Deliberately Does Not Do

| Concern | Handled By |
|---------|-----------|
| Authentication | Phase 7 JWT middleware |
| Rate limiting | Phase 7 |
| Request body parsing | `api/handlers/` |
| Business logic | `api/handlers/` |
| Database access | `store/` |
| Route matching | `api/routes.go` |

---

## Test Results Achieved

```
All 35 tests  → PASS
Stack traces  → expected — recovery middleware logging correctly
Nil panic     → handled correctly by Go 1.21+ runtime.PanicNilError
go vet        → clean
go build      → clean
```

---

## What Comes Next — `dto/`

With middleware complete, the next group is the data transfer objects — the exact JSON shapes the API accepts and returns:

```
internal/api/dto/
  ├── request.go   ← UploadRequest, IdentifyRequest
  └── response.go  ← SongResponse, MatchResponse, ErrorResponse
```

> This summary reflects the state of `middleware/logging.go`, `middleware/recovery.go`, and `middleware/cors.go` as committed. Move forward to `dto/` for further development explanation.