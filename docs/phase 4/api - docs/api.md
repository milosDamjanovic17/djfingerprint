# Phase 4 — Step 4 — `middleware/` — Introduction Document

---

## What the Middleware Package Is

Middleware is **code that wraps every HTTP request before it reaches a handler and every response before it leaves**. Each middleware in the chain does one focused thing, then passes control to the next piece in the chain.

```
Request arrives
      │
      ▼
  CORS Middleware        ← sets CORS headers
      │
      ▼
  Logging Middleware     ← records start time
      │
      ▼
  Recovery Middleware    ← wraps in panic catcher
      │
      ▼
  Handler                ← your business logic
      │
      ▼
  Recovery Middleware    ← catches any panic, returns 500
      │
      ▼
  Logging Middleware     ← records duration, status code
      │
      ▼
  CORS Middleware        ← already set headers on the way in
      │
      ▼
Response leaves
```

---

## The Three Files

### `middleware/logging.go`

Logs every HTTP request and its outcome. Records:

```
→ method, path, remote address     (on arrival)
← status code, duration, bytes     (on completion)
```

**Why logging middleware rather than logging in each handler:**
Without middleware, every handler would need to log its own start time, capture its own response code, and calculate its own duration. That is dozens of lines of duplicated boilerplate. One middleware does it once for every endpoint automatically.

**What it logs:**

```
2024/01/15 14:23:01 → POST /api/v1/upload 192.168.1.1
2024/01/15 14:23:03 ← 201 POST /api/v1/upload 2.341s 142B
```

**Implementation approach — `ResponseRecorder`:**
The standard `http.ResponseWriter` does not expose the status code or bytes written after the handler runs. The logger wraps it in a thin `responseRecorder` struct that intercepts `WriteHeader` and `Write` calls to capture these values.

```go
type responseRecorder struct {
    http.ResponseWriter
    statusCode int
    bytes      int
}
```

---

### `middleware/recovery.go`

Catches panics in handlers and converts them to HTTP 500 responses.

**Why this is necessary:**
In Go, an unrecovered panic crashes the goroutine it occurs in. For an HTTP server each request runs in its own goroutine — a panic in a handler would crash that request's goroutine, potentially leaving the connection hanging and logging a confusing stack trace. Worse, depending on the Go version and server configuration, it could bring down the entire server.

**What recovery middleware does:**

```
Without recovery:
  Handler panics → goroutine crashes → client hangs or gets connection reset

With recovery:
  Handler panics → recovery catches it → logs the stack trace → returns HTTP 500
  Server continues processing other requests normally
```

**Implementation approach — `recover()`:**
Go's built-in `recover()` function catches panics when called inside a deferred function:

```go
defer func() {
    if err := recover(); err != nil {
        // log stack trace
        // write HTTP 500
    }
}()
next.ServeHTTP(w, r)
```

---

### `middleware/cors.go`

Sets Cross-Origin Resource Sharing headers on every response so the Vue frontend running on a different port can make API calls.

**Why CORS is needed:**
Browsers enforce the Same-Origin Policy — JavaScript running on `http://localhost:5173` (Vite dev server) cannot make requests to `http://localhost:8080` (our API) unless the API explicitly permits it via CORS headers.

**What CORS middleware sets:**

```
Access-Control-Allow-Origin:  http://localhost:5173
Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
Access-Control-Max-Age:       86400
```

**Preflight requests:**
Before making a cross-origin `POST` or `DELETE`, browsers send an `OPTIONS` preflight request to check whether the actual request is allowed. The CORS middleware must respond to `OPTIONS` with `HTTP 200` and the appropriate headers — otherwise the browser blocks the actual request before it is sent.

**What it does for `OPTIONS` requests:**

```
OPTIONS /api/v1/upload
  → CORS middleware intercepts
  → responds 200 with CORS headers
  → does NOT pass to handler
  → browser satisfied, sends actual POST
```

---

## Middleware Execution Order — Why It Matters

The order middleware is registered determines the order it executes. Our order:

```
1. CORS     — must run first so preflight OPTIONS requests
               are handled before any other middleware touches them

2. Logging  — runs second so it logs every request including
               ones that hit the recovery middleware

3. Recovery — runs last (innermost) so it wraps the handler
               tightly — panics are caught before they
               unwind through logging or CORS
```

In chi this is expressed as:

```go
r.Use(cors.Handler)
r.Use(logging.Handler)
r.Use(recovery.Handler)
```

Chi executes middleware in registration order from outermost to innermost.

---

## Package Structure

```
internal/api/middleware/
  ├── logging.go
  ├── logging_test.go
  ├── recovery.go
  ├── recovery_test.go
  ├── cors.go
  └── cors_test.go
```

All three files share the same package name: `package middleware`

---

## What the Tests Will Cover

### Logging Middleware Tests
```
[ ] Request arrival is logged
[ ] Status code captured correctly (200, 201, 400, 404, 500)
[ ] Duration is measured and non-zero
[ ] Handler output reaches the client unchanged
[ ] Works for GET, POST, DELETE methods
```

### Recovery Middleware Tests
```
[ ] Panicking handler returns HTTP 500
[ ] Non-panicking handler passes through normally
[ ] Response body contains error indicator after panic
[ ] Server continues processing after a panic
[ ] String panic recovered correctly
[ ] Error panic recovered correctly
```

### CORS Middleware Tests
```
[ ] Access-Control-Allow-Origin header present on all responses
[ ] Access-Control-Allow-Methods header present
[ ] Access-Control-Allow-Headers header present
[ ] OPTIONS preflight returns HTTP 200
[ ] OPTIONS does not reach the handler
[ ] Non-OPTIONS requests pass through to handler
[ ] CORS headers present even on error responses
```

---

## What Middleware Does NOT Do

```
Does not parse request bodies     ← handlers
Does not authenticate requests    ← Phase 7 (JWT middleware)
Does not rate limit requests      ← Phase 7
Does not validate API keys        ← Phase 7
Does not touch the database       ← store layer
Does not know about songs         ← handlers
```

---

## Dependencies

```go
import (
    "log"           // standard library logging
    "net/http"      // standard library HTTP
    "runtime/debug" // stack trace in recovery middleware
    "time"          // duration measurement in logging middleware
)
```

No third-party dependencies. All three middleware files use only the standard library.

---