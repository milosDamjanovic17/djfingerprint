.PHONY: build run test vet clean

## Build the API binary into /bin
build:
	go build -o bin/djfingerprint.exe ./cmd/api

## Run the API directly without building
run:
	go run ./cmd/api

## Run all tests with verbose output
test:
	go clean -testcache
	go test ./internal/... -v

test-pkg:
	go clean -testcache
	go test $(PKG) -v

## Run tests with benchmarks
bench:
	go clean -testcache
	go test ./internal/... -bench=. -benchtime=3x -v

## Run the Go vet static analyzer
vet:
	go vet ./cmd/... ./internal/...

## Remove build artifacts
clean:
	Remove-Item -ErrorAction SilentlyContinue bin\djfingerprint.exe