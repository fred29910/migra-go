.PHONY: build test clean lint fmt vet help ci

# Build the project
build:
	go build --trimpath -ldflags="-s -w" -o migra ./cmd/migra

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
	@echo "  help        - Show this help"
