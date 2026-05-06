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

.PHONY: all build install clean clean-dist test test-race test-coverage test-compile coverage coverage-integration lint fmt fmt-check vet \
        deps tidy proto plugins plugin-github plugin-npm plugin-slack \
        test-integration test-e2e bench bench-save bench-quick bench-regression bench-memory bench-profile bench-ci bench-e2e bench-template bench-analysis \
        check-binary-size help release-local release-snapshot check check-ci install-hooks \
        frontend frontend-deps frontend-standalone build-with-frontend clean-frontend \
        test-policy-gate skill-preflight \
        mcp-apps mcp-apps-deps clean-mcp-apps \
	    sbom fuzz
	    changelog

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

# Build standalone frontend Docker image
frontend-standalone: frontend-deps
	@echo "Building standalone frontend Docker image..."
	cd $(WEB_DIR) && docker build -f docker/Dockerfile -t relicta-dashboard:latest .
	@echo "Run with: docker run -p 3000:80 relicta-dashboard:latest"

# Clean frontend artifacts
clean-frontend:
	@echo "Cleaning frontend artifacts..."
	rm -rf $(WEB_DIR)/dist $(WEB_DIR)/node_modules $(FRONTEND_DEST)

## MCP Apps targets
APP_DIR := app
MCP_APPS_DEST := internal/mcp/dist

# Install MCP app dependencies
mcp-apps-deps:
	@echo "Installing MCP app dependencies..."
	cd $(APP_DIR) && npm ci

# Build MCP apps (single-file HTML bundles embedded into Go binary)
mcp-apps: mcp-apps-deps
	@echo "Building MCP apps..."
	cd $(APP_DIR) && bash build.sh
	@echo "✓ MCP apps built and copied to $(MCP_APPS_DEST)"

# Clean MCP app artifacts
clean-mcp-apps:
	@echo "Cleaning MCP app artifacts..."
	rm -rf $(APP_DIR)/dist $(APP_DIR)/node_modules

## Test targets

# Run unit tests
test:
	@echo "Running unit tests..."
	$(GOTEST) -v ./internal/... ./pkg/...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race -v ./internal/... ./pkg/...

# Compile tests without running them (fast, deterministic, non-mutating)
test-compile:
	@echo "Compiling tests..."
	$(GOTEST) -run '^$$' ./internal/... ./pkg/...

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

# Run policy matrix gate checks (same assertions as CI policy-gate job)
test-policy-gate:
	@echo "Running policy matrix gate checks..."
	$(GOCMD) run ./cmd/relicta policy validate --file examples/policies/starter.policy
	$(GOCMD) run ./cmd/relicta policy test \
		--file examples/policies/starter.policy \
		--matrix examples/policies/policy-matrix.yaml \
		--assert-expected \
		--json >/dev/null
	@echo "✓ Policy matrix gate checks passed"

# Run skill-focused preflight checks
skill-preflight: test-policy-gate
	@echo "Validating skill package files..."
	@test -f skills/relicta-release-governance/SKILL.md
	@test -f skills/relicta-release-governance/agents/openai.yaml
	@echo "✓ Skill package files present"

# Run end-to-end tests
test-e2e:
	@echo "Running e2e tests..."
	$(GOTEST) -v -tags=e2e ./test/e2e/...

# Generate changelog from commits since latest tag
changelog:
	@echo "Generating docs/changelog.md..."
	@mkdir -p docs
	@LAST_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || true); \
	if [ -n "$$LAST_TAG" ]; then RANGE="$$LAST_TAG..HEAD"; else RANGE="HEAD"; fi; \
	{ \
		echo "# Changelog"; \
		echo ""; \
		echo "Generated on $$(date -u +"%Y-%m-%dT%H:%M:%SZ")"; \
		echo ""; \
		if [ -n "$$LAST_TAG" ]; then echo "Changes since $$LAST_TAG:"; else echo "All changes:"; fi; \
		echo ""; \
		git log $$RANGE --pretty=format:'- %s (%h)'; \
		echo ""; \
	} > docs/changelog.md
	@echo "✓ Wrote docs/changelog.md"

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

# Run benchmarks with regression comparison (requires benchstat)
bench-regression:
	@echo "Running benchmark regression comparison..."
	@mkdir -p $(BIN_DIR)
	@if [ ! -f $(BIN_DIR)/bench-baseline.txt ]; then \
		echo "No baseline found. Run 'make bench-save' first to create a baseline."; \
		echo "Creating baseline now..."; \
		$(GOTEST) -bench=. -benchmem -count=5 -run=^$$ ./internal/... ./pkg/... > $(BIN_DIR)/bench-baseline.txt; \
		echo "Baseline created at $(BIN_DIR)/bench-baseline.txt"; \
	else \
		echo "Running comparison against baseline..."; \
		$(GOTEST) -bench=. -benchmem -count=5 -run=^$$ ./internal/... ./pkg/... > $(BIN_DIR)/bench-current.txt; \
		benchstat $(BIN_DIR)/bench-baseline.txt $(BIN_DIR)/bench-current.txt; \
	fi

# Run memory-focused benchmarks
bench-memory:
	@echo "Running memory benchmarks..."
	@mkdir -p $(BIN_DIR)
	$(GOTEST) -bench=BenchmarkMemory -benchmem -run=^$$ ./internal/benchmark/... | tee $(BIN_DIR)/bench-memory.txt
	@echo "Memory benchmark results saved to $(BIN_DIR)/bench-memory.txt"

# Generate CPU and memory profiles
bench-profile:
	@echo "Running benchmarks with profiling..."
	@mkdir -p $(BIN_DIR)/profiles
	@echo "Generating CPU profile for analyzer..."
	$(GOTEST) -bench=BenchmarkAnalyzer_Analyze -benchtime=3s -cpuprofile=$(BIN_DIR)/profiles/cpu-analyzer.prof -run=^$$ ./internal/service/release/...
	@echo "Generating memory profile for analyzer..."
	$(GOTEST) -bench=BenchmarkAnalyzer_Analyze -benchtime=3s -memprofile=$(BIN_DIR)/profiles/mem-analyzer.prof -run=^$$ ./internal/service/release/...
	@echo "Generating CPU profile for notes..."
	$(GOTEST) -bench=BenchmarkGenerateNotesUseCase -benchtime=3s -cpuprofile=$(BIN_DIR)/profiles/cpu-notes.prof -run=^$$ ./internal/domain/release/app/...
	@echo "Generating memory profile for notes..."
	$(GOTEST) -bench=BenchmarkGenerateNotesUseCase -benchtime=3s -memprofile=$(BIN_DIR)/profiles/mem-notes.prof -run=^$$ ./internal/domain/release/app/...
	@echo "Generating CPU profile for e2e plan..."
	$(GOTEST) -bench=BenchmarkE2E_PlanCommand -benchtime=3s -cpuprofile=$(BIN_DIR)/profiles/cpu-e2e.prof -run=^$$ ./internal/benchmark/...
	@echo "Generating memory profile for e2e plan..."
	$(GOTEST) -bench=BenchmarkE2E_PlanCommand -benchtime=3s -memprofile=$(BIN_DIR)/profiles/mem-e2e.prof -run=^$$ ./internal/benchmark/...
	@echo ""
	@echo "Profiles saved to $(BIN_DIR)/profiles/"
	@echo "View with: go tool pprof -http=:8080 $(BIN_DIR)/profiles/<profile>.prof"

# Run e2e benchmarks specifically
bench-e2e:
	@echo "Running end-to-end benchmarks..."
	@mkdir -p $(BIN_DIR)
	$(GOTEST) -bench=BenchmarkE2E -benchmem -run=^$$ ./internal/benchmark/... | tee $(BIN_DIR)/bench-e2e.txt
	@echo "E2E benchmark results saved to $(BIN_DIR)/bench-e2e.txt"

# Run template benchmarks
bench-template:
	@echo "Running template benchmarks..."
	$(GOTEST) -bench=BenchmarkService -benchmem -run=^$$ ./internal/infrastructure/template/...

# Run analysis benchmarks (parallelization validation)
bench-analysis:
	@echo "Running analysis parallelization benchmarks..."
	@mkdir -p $(BIN_DIR)
	$(GOTEST) -bench='BenchmarkAnalyzeAll|BenchmarkAnalyze_Single' -benchmem -run=^$$ \
		./internal/analysis/... | tee $(BIN_DIR)/bench-analysis.txt
	@echo ""
	@echo "=== Parallelization Benchmarks ==="
	@grep -E '(Parallel|Sequential|parallel|seq)' $(BIN_DIR)/bench-analysis.txt || true
	@echo "==================================="

# CI-specific benchmark run (shorter, with comparison)
bench-ci:
	@echo "Running CI benchmarks..."
	@mkdir -p $(BIN_DIR)
	$(GOTEST) -bench=. -benchmem -benchtime=100ms -count=3 -run=^$$ \
		./internal/service/release/... \
		./internal/domain/release/app/... \
		./internal/benchmark/... \
		./internal/plugin/... \
		./internal/analysis/... \
		./internal/infrastructure/template/... | tee $(BIN_DIR)/bench-ci.txt
	@echo ""
	@echo "=== Performance Targets ==="
	@echo "Target: plan < 1s for 1000 commits"
	@echo "Target: notes (no AI) < 500ms"
	@echo "Target: plugin loading < 200ms"
	@echo "Target: parallelized analysis faster for I/O-bound work"
	@echo "==========================="

# Run fuzz tests for all parsers (30 seconds each)
fuzz:
	@echo "Running fuzz tests for parsers..."
	@echo "--- Fuzzing semver parser ---"
	$(GOTEST) -fuzz=FuzzParse -fuzztime=30s ./internal/domain/version/...
	@echo "--- Fuzzing version bump ---"
	$(GOTEST) -fuzz=FuzzVersionBump_Apply -fuzztime=30s ./internal/domain/version/...
	@echo "--- Fuzzing policy DSL parser ---"
	$(GOTEST) -fuzz=FuzzParsePolicy -fuzztime=30s ./internal/cgp/policy/dsl/...
	@echo "--- Fuzzing policy DSL lexer ---"
	$(GOTEST) -fuzz=FuzzLexer -fuzztime=30s ./internal/cgp/policy/dsl/...
	@echo "--- Fuzzing conventional commit parser ---"
	$(GOTEST) -fuzz=FuzzParseConventionalCommit -fuzztime=30s ./internal/infrastructure/git/...
	@echo "--- Fuzzing release type detection ---"
	$(GOTEST) -fuzz=FuzzDetectReleaseType -fuzztime=30s ./internal/infrastructure/git/...
	@echo "✓ All fuzz tests completed"

# Generate SBOM (Software Bill of Materials) in CycloneDX format
sbom: build
	@echo "Generating SBOM..."
	@command -v syft >/dev/null 2>&1 || { echo "Error: syft is not installed. Install with: brew install syft"; exit 1; }
	@mkdir -p $(BIN_DIR)
	syft dir:. -o cyclonedx-json=$(BIN_DIR)/sbom-source.cdx.json
	syft file:$(BIN_DIR)/$(BINARY_NAME) -o cyclonedx-json=$(BIN_DIR)/sbom-binary.cdx.json
	@echo "✓ SBOM generated:"
	@echo "  Source: $(BIN_DIR)/sbom-source.cdx.json"
	@echo "  Binary: $(BIN_DIR)/sbom-binary.cdx.json"

# Check binary size (target: < 20MB)
check-binary-size: build
	@echo "Checking binary size..."
	@BINARY_SIZE=$$(stat -f%z $(BIN_DIR)/$(BINARY_NAME) 2>/dev/null || stat -c%s $(BIN_DIR)/$(BINARY_NAME) 2>/dev/null); \
	BINARY_SIZE_MB=$$(echo "scale=2; $$BINARY_SIZE / 1048576" | bc); \
	echo "Binary size: $${BINARY_SIZE_MB}MB ($$BINARY_SIZE bytes)"; \
	if [ $$BINARY_SIZE -gt 20971520 ]; then \
		echo "❌ Binary size exceeds 20MB target!"; \
		exit 1; \
	else \
		echo "✓ Binary size is within 20MB target"; \
	fi

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
	@echo "  make frontend              Build the Vue frontend"
	@echo "  make frontend-deps         Install frontend dependencies"
	@echo "  make frontend-standalone   Build standalone frontend Docker image"
	@echo "  make clean-frontend        Clean frontend artifacts"
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
	@echo "  make test-policy-gate  Run policy simulation gate checks"
	@echo "  make skill-preflight   Validate skill package + policy gates"
	@echo "  make test-e2e          Run end-to-end tests"
	@echo "  make bench             Run all benchmarks"
	@echo "  make bench-save        Run benchmarks and save results"
	@echo "  make bench-quick       Run quick benchmarks (critical paths)"
	@echo "  make bench-regression  Compare benchmarks against baseline"
	@echo "  make bench-memory      Run memory-focused benchmarks"
	@echo "  make bench-profile     Generate CPU/memory profiles"
	@echo "  make bench-ci          CI-optimized benchmark run"
	@echo "  make fuzz              Run fuzz tests for all parsers (30s each)"
	@echo "  make check-binary-size Verify binary < 20MB"
	@echo ""
	@echo "Security & Compliance:"
	@echo "  make sbom              Generate SBOM (CycloneDX format)"
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
	@echo "  make check          Run fast pre-commit checks (fmt, vet, lint+gocritic, test compile)"
	@echo "  make check-ci       Run full CI checks (fmt, vet, lint, test, coverage, security)"
	@echo "  make coverage-check Run coverage threshold enforcement (coverctl)"
	@echo "  make security-scan  Run security scan (nox SAST/SCA)"
	@echo "  make install-hooks  Install git pre-commit hook"

## Pre-commit targets

# Run fast pre-commit checks (non-mutating, deterministic)
check: fmt-check vet lint test-compile
	@echo ""
	@echo "✓ All pre-commit checks passed!"

# Run full CI-equivalent checks (includes coverage enforcement)
check-ci: fmt-check vet lint test coverage-check security-scan
	@echo ""
	@echo "✓ All CI checks passed!"

# Run coverage enforcement via coverctl (fails if below thresholds)
coverage-check:
	@echo "Running coverage checks..."
	@go run github.com/felixgeelhaar/coverctl@v1.4.0 check --config .coverctl.yaml -v

# Run security scan via nox (SAST + SCA) — fails on NEW high+ findings (baseline at .nox/baseline.json suppresses pre-existing)
security-scan:
	@echo "Running security scan..."
	@if command -v nox >/dev/null 2>&1; then \
		nox scan --quiet --severity-threshold high .; \
	else \
		echo "  ERROR: nox not installed — required for CI security gate"; \
		echo "  Install: https://github.com/nox-hq/nox"; \
		exit 1; \
	fi

# AI eval harness — gates safe model bumps against the embedded golden corpus.
# V0 ships deterministic scoring (no API keys). LLM-as-judge follow-up requires
# explicit opt-in via build tag.
ai-eval:
	@echo "Running AI eval harness..."
	@go run ./cmd/relicta eval run

ai-eval-fail-fast:
	@go run ./cmd/relicta eval run --fail-fast

# Strict scan (no baseline suppression) — for periodic full audit
security-scan-strict:
	@echo "Running strict security scan (no baseline)..."
	@if command -v nox >/dev/null 2>&1; then \
		nox scan --severity-threshold high --no-baseline .; \
	else \
		echo "  ERROR: nox not installed"; \
		exit 1; \
	fi

# Install git pre-commit hook (comprehensive)
install-hooks:
	@echo "Installing git pre-commit hook..."
	@mkdir -p .git/hooks
	@cp scripts/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✓ Pre-commit hook installed successfully!"
	@echo ""
	@echo "The hook runs staged-file-aware checks:"
	@echo "  1. gofmt        — formatting verification"
	@echo "  2. go vet       — static analysis"
	@echo "  3. golangci-lint — lint + gocritic"
	@echo "  4. test compile — compile tests without running"
	@echo "  5. coverctl     — coverage threshold enforcement"
	@echo "  6. nox          — security scan (SAST/SCA)"
	@echo ""
	@echo "Run 'make check' for the same checks manually."
	@echo "Run 'make check-ci' for full CI-equivalent validation."
	@echo "Skip temporarily with: git commit --no-verify"
