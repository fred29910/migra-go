.PHONY: build test clean lint fmt vet help ci install-tools release-local release-snapshot

# Build the project
build:
	VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	GO_VERSION=$$(go version | awk '{print $$3}'); \
	go build --trimpath -ldflags="-s -w \
		-X 'github.com/fred29910/migra-go/internal/version.Version=$$VERSION' \
		-X 'github.com/fred29910/migra-go/internal/version.BuildTime=$$BUILD_TIME' \
		-X 'github.com/fred29910/migra-go/internal/version.GitCommit=$$COMMIT' \
		-X 'github.com/fred29910/migra-go/internal/version.GoVersion=$$GO_VERSION'" \
		-o migra ./cmd/migra

# Run all tests (db-less unit tests)
test:
	go test ./... -v

# Run tests with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Run go vet
vet:
	go vet ./...

# Clean build artifacts
clean:
	rm -f migra
	rm -f coverage.out coverage.html
	go clean

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format code
fmt:
	gofmt -w .
	go fmt ./...

# CI target: fmt, vet, lint, test
ci: fmt vet lint test

# Install GoReleaser CLI
install-tools:
	go install github.com/goreleaser/goreleaser@latest

# Local full release (requires GITHUB_TOKEN)
release-local:
	goreleaser release --clean

# Local snapshot build (no publish, for testing)
release-snapshot:
	goreleaser release --snapshot --clean

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build the project"
	@echo "  test        - Run all tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  vet         - Run go vet"
	@echo "  clean       - Clean build artifacts"
	@echo "  lint        - Run linter (golangci-lint)"
	@echo "  fmt         - Format code (gofmt + go fmt)"
	@echo "  ci          - Run CI checks (fmt, vet, lint, test)"
	@echo "  install-tools - Install GoReleaser CLI"
	@echo "  release-local - Run goreleaser release (full, requires GITHUB_TOKEN)"
	@echo "  release-snapshot - Run goreleaser release --snapshot (local test)"
	@echo "  help        - Show this help"
