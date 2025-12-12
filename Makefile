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
DIST_DIR := dist
CMD_DIR := cmd/release-pilot
PLUGINS_DIR := plugins

# All plugin binaries (matching GoReleaser config)
ALL_PLUGINS := github gitlab npm slack discord jira launchnotes

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

.PHONY: all build install clean clean-dist test test-race test-coverage lint fmt vet \
        deps tidy proto plugins plugin-github plugin-npm plugin-slack \
        test-integration test-e2e help release-build release-binaries release-plugins \
        release-archives release-checksums release-snapshot

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

## Release build targets (replacement for GoReleaser)

# Full release build - creates everything GoReleaser would create
release-build: clean-dist release-binaries release-plugins release-archives release-checksums
	@echo "✓ Release build complete!"
	@echo ""
	@echo "Artifacts in $(DIST_DIR):"
	@ls -lh $(DIST_DIR)/*.tar.gz $(DIST_DIR)/*.zip 2>/dev/null || true
	@echo ""
	@echo "Plugin binaries:"
	@ls -1 $(DIST_DIR)/*_linux_* $(DIST_DIR)/*_darwin_* $(DIST_DIR)/*_windows_* 2>/dev/null | head -10
	@echo "... and more"

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
release-plugins:
	@echo "Building plugins for all platforms..."
	@mkdir -p $(DIST_DIR)
	@$(foreach plugin,$(ALL_PLUGINS), \
		$(foreach platform,$(RELEASE_PLATFORMS), \
			$(eval OS := $(word 1,$(subst /, ,$(platform)))) \
			$(eval ARCH := $(word 2,$(subst /, ,$(platform)))) \
			$(eval ARCH_NAME := $(call get_arch_name,$(ARCH))) \
			$(eval EXT := $(if $(filter windows,$(OS)),.exe,)) \
			$(eval PLUGIN_BIN := $(plugin)_$(OS)_$(ARCH_NAME)$(EXT)) \
			echo "  Building $(plugin) for $(OS)/$(ARCH)..." && \
			GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 \
				$(GOBUILD) -ldflags "-s -w" \
				-o $(DIST_DIR)/$(PLUGIN_BIN) \
				./$(PLUGINS_DIR)/$(plugin) || exit 1; \
		) \
	)

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
	@echo "ReleasePilot Build Commands"
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
