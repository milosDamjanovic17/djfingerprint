# dependencies needed:
### PostgreSQL driver for Go
go get github.com/lib/pq

### Chi router - lightweight idiomatic HTTP router
go get github.com/go-chi/chi/v5

### Chi middleware package
go get github.com/go-chi/chi/v5/middleware

### CORS middleware
go get github.com/go-chi/cors

### Environment variable loading from .env file
go get github.com/joho/godotenv

### Gonum DSP for FFT (Phase 3, install now to avoid interruption later)
go get gonum.org/v1/gonum/dsp/fourier

### Gonum math utilities
go get gonum.org/v1/gonum/floats

### UUID generation (for any future need)
go get github.com/google/uuid

### Tidy up go.mod and go.sum
go mod tidy