.PHONY: build test clean lint fmt help

# Build the project
build:
	go build -o schemadiff ./cmd/schemadiff

# Run all tests
test:
	go test ./... -v

# Run tests with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f schemadiff
	rm -f coverage.out coverage.html
	go clean

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build the project"
	@echo "  test        - Run all tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean       - Clean build artifacts"
	@echo "  lint        - Run linter"
	@echo "  fmt         - Format code"
	@echo "  help        - Show this help"
