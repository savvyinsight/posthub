.PHONY: build test lint clean run-api run-worker fmt vet

# Build variables
API_BINARY    := posthub-api
WORKER_BINARY := posthub-worker
BUILD_DIR     := ./bin
GO            := go
GOFLAGS       := -trimpath

# Build both binaries
build: build-api build-worker

build-api:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(API_BINARY) ./cmd/api

build-worker:
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(WORKER_BINARY) ./cmd/worker

# Run the API server
run-api:
	$(GO) run ./cmd/api

# Run the worker
run-worker:
	$(GO) run ./cmd/worker

# Run all tests
test:
	$(GO) test ./... -v -count=1

# Run tests with race detector
test-race:
	$(GO) test ./... -v -race -count=1

# Run tests with coverage
test-cover:
	$(GO) test ./... -cover -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GO) fmt ./...

# Run go vet
vet:
	$(GO) vet ./...

# Tidy dependencies
tidy:
	$(GO) mod tidy

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

# Check everything (CI target)
check: fmt vet test

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build both API and worker binaries"
	@echo "  build-api    - Build API binary only"
	@echo "  build-worker - Build worker binary only"
	@echo "  run-api      - Run API server locally"
	@echo "  run-worker   - Run worker locally"
	@echo "  test         - Run all tests"
	@echo "  test-race    - Run tests with race detector"
	@echo "  test-cover   - Run tests with coverage report"
	@echo "  lint         - Run golangci-lint"
	@echo "  fmt          - Format code with gofmt"
	@echo "  vet          - Run go vet"
	@echo "  tidy         - Tidy go.mod dependencies"
	@echo "  clean        - Remove build artifacts"
	@echo "  check        - Run fmt, vet, and test"
