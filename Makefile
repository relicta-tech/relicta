# Relicta Makefile
# Build automation for the relicta CLI

# Variables
BINARY_NAME := relicta
MODULE := github.com/relicta-tech/relicta
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.ver=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

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
DIST_DIR := dist
CMD_DIR := cmd/relicta

.PHONY: all build install clean clean-dist test test-race test-coverage coverage coverage-integration lint fmt fmt-check vet \
        deps tidy proto plugins plugin-github plugin-npm plugin-slack \
        test-integration test-e2e bench bench-save bench-quick help release-local release-snapshot check install-hooks \
        frontend frontend-deps build-with-frontend clean-frontend

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

## Frontend targets

# Frontend directories
WEB_DIR := web
FRONTEND_DEST := $(CMD_DIR)/frontend

# Install frontend dependencies
frontend-deps:
	@echo "Installing frontend dependencies..."
	cd $(WEB_DIR) && npm ci

# Build frontend
frontend: frontend-deps
	@echo "Building frontend..."
	cd $(WEB_DIR) && npm run build
	@echo "Copying frontend to $(FRONTEND_DEST)..."
	rm -rf $(FRONTEND_DEST)
	mkdir -p $(FRONTEND_DEST)
	cp -r $(WEB_DIR)/dist/* $(FRONTEND_DEST)/
	@echo "✓ Frontend built and copied to $(FRONTEND_DEST)"

# Build binary with embedded frontend
build-with-frontend: frontend
	@echo "Building $(BINARY_NAME) with embedded frontend..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -tags embed_frontend -o $(BIN_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "✓ Built $(BIN_DIR)/$(BINARY_NAME) with embedded frontend"

# Clean frontend artifacts
clean-frontend:
	@echo "Cleaning frontend artifacts..."
	rm -rf $(WEB_DIR)/dist $(WEB_DIR)/node_modules $(FRONTEND_DEST)

## Test targets

# Run unit tests
test:
	@echo "Running unit tests..."
	$(GOTEST) -v ./internal/... ./pkg/...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./internal/... ./pkg/...

# Run tests with coverage (simple)
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(BIN_DIR)
	$(GOTEST) -coverprofile=$(BIN_DIR)/coverage.out -covermode=atomic ./internal/... ./pkg/...
	$(GOCMD) tool cover -html=$(BIN_DIR)/coverage.out -o $(BIN_DIR)/coverage.html
	@echo "Coverage report generated at $(BIN_DIR)/coverage.html"

# Run tests with coverage enforcement via coverctl
coverage:
	@echo "Running coverage checks with coverctl..."
	go run github.com/felixgeelhaar/coverctl@v1.4.0 check --race -v

# Run coverage with integration tests
coverage-integration:
	@echo "Running coverage with integration tests..."
	go run github.com/felixgeelhaar/coverctl@v1.4.0 check --race --tags integration -v

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./test/integration/...

# Run end-to-end tests
test-e2e:
	@echo "Running e2e tests..."
	$(GOTEST) -v -tags=e2e ./test/e2e/...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem -run=^$$ ./internal/... ./pkg/...

# Run benchmarks and save results
bench-save:
	@echo "Running benchmarks and saving results..."
	@mkdir -p $(BIN_DIR)
	$(GOTEST) -bench=. -benchmem -run=^$$ ./internal/... ./pkg/... | tee $(BIN_DIR)/bench-results.txt
	@echo "Results saved to $(BIN_DIR)/bench-results.txt"

# Run quick benchmarks (critical paths only)
bench-quick:
	@echo "Running quick benchmarks..."
	$(GOTEST) -bench=. -benchmem -benchtime=100ms -run=^$$ \
		./internal/infrastructure/git/... \
		./internal/domain/version/... \
		./internal/plugin/...

## Code quality targets

# Run linter
lint:
	@echo "Running linter..."
	$(GOLINT) run ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Check formatting (no changes, just verify)
fmt-check:
	@echo "Checking code formatting..."
	@DIFF=$$($(GOFMT) -s -l .); \
	if [ -n "$$DIFF" ]; then \
		echo "❌ The following files need formatting:"; \
		echo "$$DIFF"; \
		echo ""; \
		echo "Run 'make fmt' to fix formatting."; \
		exit 1; \
	fi
	@echo "✓ All files are properly formatted"

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

## Release targets (using GoReleaser)
# Relicta governs releases - GoReleaser handles the actual builds

# Local snapshot build for testing (no signing, no publish)
release-local:
	@echo "Building local snapshot with GoReleaser..."
	goreleaser release --snapshot --clean --skip=sign,publish
	@echo ""
	@echo "✓ Local build complete! Artifacts in $(DIST_DIR)/"

# Full snapshot build (includes signing if cosign is available)
release-snapshot:
	@echo "Building snapshot release..."
	goreleaser release --snapshot --clean --skip=publish

## Utility targets

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out

# Clean dist directory
clean-dist:
	@echo "Cleaning dist directory..."
	rm -rf $(DIST_DIR)

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
	@echo "Relicta Build Commands"
	@echo ""
	@echo "Build:"
	@echo "  make build               Build the binary (API-only)"
	@echo "  make build-with-frontend Build with embedded Vue frontend"
	@echo "  make build-all           Build for all platforms"
	@echo "  make install             Install to GOPATH/bin"
	@echo "  make plugins             Build all plugins"
	@echo ""
	@echo "Frontend:"
	@echo "  make frontend            Build the Vue frontend"
	@echo "  make frontend-deps       Install frontend dependencies"
	@echo "  make clean-frontend      Clean frontend artifacts"
	@echo ""
	@echo "Release (via GoReleaser):"
	@echo "  make release-local     Local snapshot build (no signing, no publish)"
	@echo "  make release-snapshot  Full snapshot build (includes signing)"
	@echo ""
	@echo "Test:"
	@echo "  make test              Run unit tests"
	@echo "  make test-race         Run tests with race detection"
	@echo "  make test-coverage     Run tests with coverage report"
	@echo "  make coverage          Run coverage with policy enforcement (coverctl)"
	@echo "  make coverage-integration  Run coverage including integration tests"
	@echo "  make test-integration  Run integration tests"
	@echo "  make test-e2e          Run end-to-end tests"
	@echo "  make bench             Run all benchmarks"
	@echo "  make bench-save        Run benchmarks and save results"
	@echo "  make bench-quick       Run quick benchmarks (critical paths)"
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
	@echo "  make clean-dist     Clean dist directory"
	@echo "  make version        Show version info"
	@echo ""
	@echo "Pre-commit:"
	@echo "  make check          Run all pre-commit checks (fmt, lint, vet, test)"
	@echo "  make install-hooks  Install git pre-commit hook"

## Pre-commit targets

# Run all pre-commit checks (mirrors CI workflow)
check: fmt-check vet lint test
	@echo ""
	@echo "✓ All pre-commit checks passed!"

# Install git pre-commit hook
install-hooks:
	@echo "Installing git pre-commit hook..."
	@mkdir -p .git/hooks
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo '# Pre-commit hook - runs make check before each commit' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo 'echo "Running pre-commit checks..."' >> .git/hooks/pre-commit
	@echo 'make check' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo 'if [ $$? -ne 0 ]; then' >> .git/hooks/pre-commit
	@echo '    echo ""' >> .git/hooks/pre-commit
	@echo '    echo "❌ Pre-commit checks failed. Please fix the issues above."' >> .git/hooks/pre-commit
	@echo '    echo "   Run '\''make check'\'' to see all issues."' >> .git/hooks/pre-commit
	@echo '    echo "   Run '\''make fmt'\'' to auto-fix formatting."' >> .git/hooks/pre-commit
	@echo '    exit 1' >> .git/hooks/pre-commit
	@echo 'fi' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✓ Pre-commit hook installed successfully!"
	@echo ""
	@echo "The hook will run: make check (fmt-check, vet, lint, test)"
	@echo "To skip the hook temporarily: git commit --no-verify"
