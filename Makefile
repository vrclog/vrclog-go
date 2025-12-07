.PHONY: all build test test-race test-cover lint clean release-snapshot help

# Default target
all: lint test build

# Build the CLI binary
build:
	go build -o vrclog ./cmd/vrclog

# Build for Windows (cross-compile)
build-windows:
	GOOS=windows GOARCH=amd64 go build -o vrclog.exe ./cmd/vrclog

# Run tests
test:
	go test ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run tests with coverage
test-cover:
	go test -cover ./...
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Install: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Tidy dependencies
tidy:
	go mod tidy

# Clean build artifacts
clean:
	rm -f vrclog vrclog.exe
	rm -f coverage.out coverage.html
	rm -rf dist/

# Local release test (requires goreleaser)
release-snapshot:
	@which goreleaser > /dev/null || (echo "goreleaser not found. Install: https://goreleaser.com/install/" && exit 1)
	goreleaser release --snapshot --clean

# Show help
help:
	@echo "Available targets:"
	@echo "  all             - Run lint, test, and build (default)"
	@echo "  build           - Build the CLI binary"
	@echo "  build-windows   - Cross-compile for Windows"
	@echo "  test            - Run tests"
	@echo "  test-race       - Run tests with race detector"
	@echo "  test-cover      - Run tests with coverage report"
	@echo "  lint            - Run golangci-lint"
	@echo "  fmt             - Format code"
	@echo "  vet             - Run go vet"
	@echo "  tidy            - Tidy dependencies"
	@echo "  clean           - Clean build artifacts"
	@echo "  release-snapshot- Test release locally"
	@echo "  help            - Show this help"
