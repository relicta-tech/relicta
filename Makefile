# Relicta Makefile
# Build automation for the relicta CLI

# Variables
BINARY_NAME := relicta
MODULE := github.com/relicta-tech/relicta
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
DIST_DIR := dist
CMD_DIR := cmd/relicta
PLUGINS_DIR := plugins

# All plugin binaries - NOTE: Plugins are now in separate repositories (relicta-tech/plugin-*)
# This is kept for backwards compatibility with local development only
ALL_PLUGINS :=

# Release platforms (os/arch pairs)
RELEASE_PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Arch mapping for binary naming (amd64 -> x86_64, arm64 -> aarch64)
define get_arch_name
$(if $(filter amd64,$(1)),x86_64,$(if $(filter arm64,$(1)),aarch64,$(1)))
endef

# OS name capitalization for archives (match GoReleaser: Linux, Darwin, Windows)
define get_os_name
$(if $(filter linux,$(1)),Linux,$(if $(filter darwin,$(1)),Darwin,$(if $(filter windows,$(1)),Windows,$(1))))
endef

.PHONY: all build install clean clean-dist test test-race test-coverage lint fmt fmt-check vet \
        deps tidy proto plugins plugin-github plugin-npm plugin-slack \
        test-integration test-e2e help release-build release-binaries release-plugins \
        release-archives release-checksums release-snapshot check install-hooks

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
# NOTE: Plugins are now in separate repositories (relicta-tech/plugin-*)
# These targets are kept for backwards compatibility but now show guidance

# Build all plugins (no-op - plugins are in separate repos)
plugins:
	@echo "Plugins are now in separate repositories:"
	@echo "  - github:  https://github.com/relicta-tech/plugin-github"
	@echo "  - gitlab:  https://github.com/relicta-tech/plugin-gitlab"
	@echo "  - npm:     https://github.com/relicta-tech/plugin-npm"
	@echo "  - slack:   https://github.com/relicta-tech/plugin-slack"
	@echo "  - discord: https://github.com/relicta-tech/plugin-discord"
	@echo "  - jira:    https://github.com/relicta-tech/plugin-jira"
	@echo ""
	@echo "Install plugins with: relicta plugin install <name>"

plugin-github plugin-npm plugin-slack:
	@echo "Plugin targets are deprecated. Use 'relicta plugin install <name>' instead."

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

## Release build targets (replacement for GoReleaser)

# Full release build - creates everything GoReleaser would create
# NOTE: Plugins are now built separately in their own repositories (relicta-tech/plugin-*)
release-build: clean-dist release-binaries release-archives release-checksums
	@echo "✓ Release build complete!"
	@echo ""
	@echo "Artifacts in $(DIST_DIR):"
	@ls -lh $(DIST_DIR)/*.tar.gz $(DIST_DIR)/*.zip 2>/dev/null || true

# Build main binary for all release platforms
release-binaries:
	@echo "Building release binaries for all platforms..."
	@mkdir -p $(DIST_DIR)
	@$(foreach platform,$(RELEASE_PLATFORMS), \
		$(eval OS := $(word 1,$(subst /, ,$(platform)))) \
		$(eval ARCH := $(word 2,$(subst /, ,$(platform)))) \
		$(eval OS_CAP := $(call get_os_name,$(OS))) \
		$(eval ARCH_NAME := $(call get_arch_name,$(ARCH))) \
		$(eval ARCHIVE_NAME := $(BINARY_NAME)_$(OS_CAP)_$(ARCH_NAME)) \
		$(eval EXT := $(if $(filter windows,$(OS)),.exe,)) \
		echo "  Building $(OS)/$(ARCH)..." && \
		mkdir -p $(DIST_DIR)/$(ARCHIVE_NAME) && \
		GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 \
			$(GOBUILD) $(LDFLAGS) \
			-o $(DIST_DIR)/$(ARCHIVE_NAME)/$(BINARY_NAME)$(EXT) \
			./$(CMD_DIR) || exit 1; \
	)

# Build all plugins for all release platforms
# NOTE: Plugins are now in separate repositories (relicta-tech/plugin-*)
# This target is kept for backwards compatibility but is a no-op
release-plugins:
	@echo "Skipping plugin builds (plugins are now in separate repositories)"
	@echo "See: https://github.com/relicta-tech/plugin-*"

# Create archives (tar.gz for linux/darwin, zip for windows)
release-archives:
	@echo "Creating release archives..."
	@$(foreach platform,$(RELEASE_PLATFORMS), \
		$(eval OS := $(word 1,$(subst /, ,$(platform)))) \
		$(eval ARCH := $(word 2,$(subst /, ,$(platform)))) \
		$(eval OS_CAP := $(call get_os_name,$(OS))) \
		$(eval ARCH_NAME := $(call get_arch_name,$(ARCH))) \
		$(eval ARCHIVE_NAME := $(BINARY_NAME)_$(OS_CAP)_$(ARCH_NAME)) \
		$(eval EXT := $(if $(filter windows,$(OS)),.exe,)) \
		echo "  Creating archive for $(OS)/$(ARCH)..." && \
		cp README.md LICENSE CHANGELOG.md $(DIST_DIR)/$(ARCHIVE_NAME)/ 2>/dev/null || true && \
		(cd $(DIST_DIR) && \
			if [ "$(OS)" = "windows" ]; then \
				zip -q -r $(ARCHIVE_NAME).zip $(ARCHIVE_NAME); \
			else \
				tar czf $(ARCHIVE_NAME).tar.gz $(ARCHIVE_NAME); \
			fi \
		) && \
		rm -rf $(DIST_DIR)/$(ARCHIVE_NAME) || exit 1; \
	)

# Generate checksums file
release-checksums:
	@echo "Generating checksums..."
	@cd $(DIST_DIR) && \
		sha256sum *.tar.gz *.zip 2>/dev/null > checksums.txt || \
		shasum -a 256 *.tar.gz *.zip 2>/dev/null > checksums.txt
	@echo "✓ Checksums generated: $(DIST_DIR)/checksums.txt"

# Build snapshot release (without version tag)
release-snapshot: clean-dist
	@echo "Building snapshot release..."
	@$(MAKE) VERSION="$(VERSION)-snapshot" release-build

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
	@echo "  make build          Build the binary"
	@echo "  make build-all      Build for all platforms"
	@echo "  make install        Install to GOPATH/bin"
	@echo "  make plugins        Build all plugins"
	@echo ""
	@echo "Release Build (replaces GoReleaser):"
	@echo "  make release-build     Full release build (binaries + plugins + archives + checksums)"
	@echo "  make release-snapshot  Build snapshot release"
	@echo "  make release-binaries  Build main binary for all platforms"
	@echo "  make release-plugins   Build all plugins for all platforms"
	@echo "  make release-archives  Create release archives"
	@echo "  make release-checksums Generate checksums"
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
