# ReleasePilot Makefile
# Build automation for the release-pilot CLI

# Variables
BINARY_NAME := release-pilot
MODULE := github.com/felixgeelhaar/release-pilot
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go commands
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOLINT := golangci-lint

# Directories
BIN_DIR := bin
CMD_DIR := cmd/release-pilot
PLUGINS_DIR := plugins

# Plugin binaries
PLUGINS := github npm slack

.PHONY: all build install clean test test-race test-coverage lint fmt vet \
        deps tidy proto plugins plugin-github plugin-npm plugin-slack \
        test-integration test-e2e help

# Default target
all: lint test build

## Build targets

# Build the main binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

# Build for all platforms
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)

build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe ./$(CMD_DIR)

# Install binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) ./$(CMD_DIR)

## Plugin targets

# Build all plugins
plugins: plugin-github plugin-npm plugin-slack

plugin-github:
	@echo "Building GitHub plugin..."
	@mkdir -p $(BIN_DIR)/plugins
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/plugins/release-pilot-github ./$(PLUGINS_DIR)/github

plugin-npm:
	@echo "Building npm plugin..."
	@mkdir -p $(BIN_DIR)/plugins
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/plugins/release-pilot-npm ./$(PLUGINS_DIR)/npm

plugin-slack:
	@echo "Building Slack plugin..."
	@mkdir -p $(BIN_DIR)/plugins
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/plugins/release-pilot-slack ./$(PLUGINS_DIR)/slack

## Test targets

# Run unit tests
test:
	@echo "Running unit tests..."
	$(GOTEST) -v ./internal/... ./pkg/...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./internal/... ./pkg/...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(BIN_DIR)
	$(GOTEST) -coverprofile=$(BIN_DIR)/coverage.out -covermode=atomic ./internal/... ./pkg/...
	$(GOCMD) tool cover -html=$(BIN_DIR)/coverage.out -o $(BIN_DIR)/coverage.html
	@echo "Coverage report generated at $(BIN_DIR)/coverage.html"

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./test/integration/...

# Run end-to-end tests
test-e2e:
	@echo "Running e2e tests..."
	$(GOTEST) -v -tags=e2e ./test/e2e/...

## Code quality targets

# Run linter
lint:
	@echo "Running linter..."
	$(GOLINT) run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## Dependency targets

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Update dependencies
update:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

## Proto targets

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/plugin/proto/*.proto

## Utility targets

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out

# Show version info
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"

# Generate mocks for testing
mocks:
	@echo "Generating mocks..."
	mockery --all --dir=internal --output=internal/mocks --outpkg=mocks

# Help
help:
	@echo "ReleasePilot Build Commands"
	@echo ""
	@echo "Build:"
	@echo "  make build          Build the binary"
	@echo "  make build-all      Build for all platforms"
	@echo "  make install        Install to GOPATH/bin"
	@echo "  make plugins        Build all plugins"
	@echo ""
	@echo "Test:"
	@echo "  make test           Run unit tests"
	@echo "  make test-race      Run tests with race detection"
	@echo "  make test-coverage  Run tests with coverage report"
	@echo "  make test-integration Run integration tests"
	@echo "  make test-e2e       Run end-to-end tests"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint           Run golangci-lint"
	@echo "  make fmt            Format code"
	@echo "  make vet            Run go vet"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps           Download dependencies"
	@echo "  make tidy           Tidy go.mod"
	@echo "  make update         Update dependencies"
	@echo ""
	@echo "Other:"
	@echo "  make proto          Generate protobuf code"
	@echo "  make mocks          Generate test mocks"
	@echo "  make clean          Clean build artifacts"
	@echo "  make version        Show version info"
