# Test Coverage Improvement Tracking

**Created:** 2025-12-13
**Branch:** planning/phase-2
**Target Release:** Phase 2A
**Status:** TRACKING

## Overview

Following the multi-agent code review, several critical test coverage gaps were identified that must be addressed before merging to main. This document tracks the required improvements.

**Overall Coverage Target:** 70%+ for critical components, 80%+ for core business logic

---

## Priority 1: Critical Coverage Gaps (BEFORE MERGE)

These components have 0% or critically low coverage and must be tested before release.

### Issue 1: Wizard UI Test Coverage
**Component:** `internal/ui/wizard/`
**Current Coverage:** 0%
**Target Coverage:** 70%+
**Priority:** P0 - BLOCKER
**Estimated Effort:** 2-3 days

#### Files Requiring Tests:
- `wizard.go` (289 lines) - Main orchestration
- `welcome.go` - Welcome screen
- `detection.go` - Detection screen
- `template.go` - Template selection
- `review.go` - Review & preview
- `success.go` - Success screen

#### Test Requirements:
- ✅ Unit tests for each screen model
- ✅ State transition testing (Welcome → Detection → Template → Review → Success)
- ✅ Error handling for failed state transitions
- ✅ User input validation
- ✅ Configuration building and saving
- ✅ File permission verification (0600)

#### Key Test Scenarios:
```go
// Test wizard flow completion
func TestWizard_CompleteFlow(t *testing.T) {
    // Test successful completion from welcome to success
}

// Test quit at each stage
func TestWizard_QuitAtEachStage(t *testing.T) {
    // Test quitting at welcome, detection, template, review
}

// Test error handling
func TestWizard_ErrorHandling(t *testing.T) {
    // Test detection failure, template building failure, save failure
}

// Test state transitions
func TestWizard_StateTransitions(t *testing.T) {
    // Test valid and invalid state transitions
}

// Test configuration building
func TestWizard_ConfigurationBuilding(t *testing.T) {
    // Test template + detection + user input merging
}
```

#### Acceptance Criteria:
- [ ] 70%+ line coverage for wizard package
- [ ] All state transitions tested
- [ ] Error paths tested
- [ ] Integration test for full wizard flow
- [ ] Configuration output validated

---

### Issue 2: Template Detector Test Coverage
**Component:** `internal/cli/templates/detector.go`
**Current Coverage:** 0%
**Target Coverage:** 80%+
**Priority:** P0 - BLOCKER
**Estimated Effort:** 1.5-2 days

#### Test Requirements:
- ✅ Language detection accuracy tests
- ✅ Platform detection tests
- ✅ Project type classification tests
- ✅ Template suggestion tests
- ✅ Confidence scoring validation
- ✅ Edge cases (monorepo, multi-language projects)

#### Key Test Scenarios:
```go
// Test Go project detection
func TestDetector_DetectGoProject(t *testing.T) {
    // Test go.mod, main.go, cmd/ detection
}

// Test Node project detection
func TestDetector_DetectNodeProject(t *testing.T) {
    // Test package.json, .js/.ts files
}

// Test monorepo detection
func TestDetector_DetectMonorepo(t *testing.T) {
    // Test packages/, apps/, lerna.json, pnpm-workspace.yaml
}

// Test Docker/Kubernetes detection
func TestDetector_DetectContainerPlatform(t *testing.T) {
    // Test Dockerfile, docker-compose.yml, k8s/
}

// Test template suggestion
func TestDetector_SuggestTemplate(t *testing.T) {
    // Test suggestion logic based on detection results
}

// Test confidence scoring
func TestDetector_ConfidenceScoring(t *testing.T) {
    // Test score calculation for language/platform/type
}
```

#### Test Data Structure:
```
testdata/
├── go-cli/               # Go CLI project fixture
│   ├── go.mod
│   ├── main.go
│   └── cmd/
├── node-web/             # Node.js web project fixture
│   ├── package.json
│   └── src/
├── monorepo/             # Monorepo fixture
│   ├── packages/
│   └── apps/
├── docker-k8s/           # Container project fixture
│   ├── Dockerfile
│   └── k8s/
└── multi-language/       # Multi-language project fixture
    ├── go.mod
    └── package.json
```

#### Acceptance Criteria:
- [ ] 80%+ line coverage for detector package
- [ ] Detection accuracy >90% for primary language
- [ ] All project types covered
- [ ] Monorepo detection working
- [ ] Edge cases handled (multi-language, missing files)

---

### Issue 3: Gemini AI Provider Test Coverage
**Component:** `internal/service/ai/gemini.go`
**Current Coverage:** ~18%
**Target Coverage:** 80%+
**Priority:** P0 - BLOCKER
**Estimated Effort:** 1-1.5 days

#### Test Requirements:
- ✅ Successful request/response handling
- ✅ Error handling (network, API errors, rate limits)
- ✅ Request building and model mapping
- ✅ Response parsing
- ✅ Integration tests with mock server
- ✅ Timeout handling

#### Key Test Scenarios:
```go
// Test successful changelog generation
func TestGemini_GenerateChangelog(t *testing.T) {
    // Mock Gemini API response
    // Test changelog generation with commits
}

// Test error handling
func TestGemini_ErrorHandling(t *testing.T) {
    // Test 401, 429, 500 responses
    // Test timeout
    // Test malformed response
}

// Test model mapping
func TestGemini_ModelMapping(t *testing.T) {
    // Test gemini-2.0-flash-exp, gemini-1.5-pro, etc.
}

// Test request building
func TestGemini_RequestBuilding(t *testing.T) {
    // Test context limits, token limits, temperature
}

// Test availability check
func TestGemini_IsAvailable(t *testing.T) {
    // Test with valid/invalid API key
}
```

#### Mock Strategy:
```go
// Use httptest for mock Gemini server
mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Validate request
    // Return mock response
}))
defer mockServer.Close()

// Configure service with mock URL
service := NewGeminiService(
    WithAPIKey("test-key"),
    WithBaseURL(mockServer.URL),
)
```

#### Acceptance Criteria:
- [ ] 80%+ line coverage for gemini package
- [ ] All API methods tested
- [ ] Error paths covered
- [ ] Integration tests with mock server
- [ ] Timeout and retry logic tested

---

## Priority 2: Medium Coverage Improvements (7-10 days post-merge)

### Issue 4: Template System Test Coverage
**Components:**
- `internal/cli/templates/registry.go`
- `internal/cli/templates/builder.go`
- `internal/cli/templates/questions.go`

**Current Coverage:** 0%
**Target Coverage:** 75%+
**Priority:** P1
**Estimated Effort:** 2-3 days

#### Test Requirements:
- ✅ Template registration and discovery
- ✅ Configuration building from templates
- ✅ Question validation and handling
- ✅ Template rendering with Go templates
- ✅ Variable substitution

---

## Priority 3: General Coverage Improvements (ongoing)

### Issue 5: Overall Package Coverage
**Target:** Minimum 70% average coverage across all packages

#### Packages Below Target:
1. `internal/ui/wizard/` - 0% → 70%+ (P0)
2. `internal/cli/templates/` - 0% → 75%+ (P1)
3. `internal/service/ai/gemini.go` - 18% → 80%+ (P0)
4. `pkg/plugin/` - Review and improve
5. `internal/container/` - Review and improve

---

## Testing Standards

### Required Test Types:
1. **Unit Tests** - Test individual functions and methods
2. **Integration Tests** - Test component interactions
3. **Table-Driven Tests** - Use for multiple scenarios
4. **Mock/Stub Usage** - For external dependencies (HTTP, filesystem)
5. **Error Path Testing** - Cover all error conditions

### Coverage Measurement:
```bash
# Run tests with coverage
make test

# View coverage report
go tool cover -html=coverage.out

# Check coverage by package
go test -coverprofile=coverage.out ./... && \
go tool cover -func=coverage.out | grep -E "^(total|.*templates|.*wizard|.*gemini)"
```

### CI/CD Integration:
- [ ] Add coverage check to GitHub Actions
- [ ] Fail build if coverage drops below 60%
- [ ] Report coverage on PRs
- [ ] Track coverage trends over time

---

## Implementation Plan

### Week 1: Critical Blockers
**Days 1-2:** Wizard UI tests (P0)
**Days 3-4:** Template detector tests (P0)
**Day 5:** Gemini provider tests (P0)

### Week 2: Medium Priority
**Days 1-3:** Template system tests (P1)
**Days 4-5:** Overall coverage improvements

---

## Success Metrics

### Before Merge (Phase 2A Release):
- ✅ Wizard package: 70%+ coverage
- ✅ Detector package: 80%+ coverage
- ✅ Gemini package: 80%+ coverage
- ✅ Overall: 61% → 65%+ coverage
- ✅ All P0 tests passing

### Post-Merge (1 week):
- ✅ Template system: 75%+ coverage
- ✅ Overall: 65% → 70%+ coverage
- ✅ All P1 tests passing

### Long-term (Ongoing):
- ✅ Overall: 70%+ coverage maintained
- ✅ No new code below 60% coverage
- ✅ Critical paths: 90%+ coverage

---

## Issue Creation Commands

When ready to create GitHub issues, use these templates:

### Issue 1: Wizard UI Test Coverage
```markdown
**Title:** Add comprehensive test coverage for Wizard UI (0% → 70%+)

**Labels:** testing, priority-blocker, phase-2a

**Description:**
The wizard UI package (`internal/ui/wizard/`) has 0% test coverage and must be tested before Phase 2A release.

**Requirements:**
- Test all screen models (welcome, detection, template, review, success)
- Test state transitions
- Test error handling
- Test configuration building and saving
- Target: 70%+ line coverage

**Acceptance Criteria:**
- [ ] 70%+ line coverage for wizard package
- [ ] All state transitions tested
- [ ] Integration test for full wizard flow
- [ ] All tests passing in CI

**Related:** docs/test-coverage-tracking.md
```

### Issue 2: Template Detector Test Coverage
```markdown
**Title:** Add comprehensive test coverage for Template Detector (0% → 80%+)

**Labels:** testing, priority-blocker, phase-2a

**Description:**
The template detector (`internal/cli/templates/detector.go`) has 0% test coverage.

**Requirements:**
- Test language detection (Go, Node, Python, Rust, Ruby)
- Test platform detection (Docker, Kubernetes, Serverless)
- Test project type classification
- Test monorepo detection
- Target: 80%+ line coverage

**Acceptance Criteria:**
- [ ] 80%+ line coverage
- [ ] Detection accuracy >90%
- [ ] All project types covered
- [ ] Edge cases handled

**Related:** docs/test-coverage-tracking.md
```

### Issue 3: Gemini Provider Test Coverage
```markdown
**Title:** Add comprehensive test coverage for Gemini AI Provider (~18% → 80%+)

**Labels:** testing, priority-blocker, phase-2a, ai-providers

**Description:**
The Gemini AI provider has only ~18% test coverage.

**Requirements:**
- Test all AI generation methods
- Test error handling (network, API, rate limits)
- Test request building and model mapping
- Integration tests with mock server
- Target: 80%+ line coverage

**Acceptance Criteria:**
- [ ] 80%+ line coverage
- [ ] All API methods tested
- [ ] Error paths covered
- [ ] Mock server integration tests

**Related:** docs/test-coverage-tracking.md
```

---

## Notes

- All P0 issues are blockers for Phase 2A merge
- Coverage targets are minimums - higher is better
- Use table-driven tests for multiple scenarios
- Mock external dependencies (HTTP, filesystem, AI APIs)
- Document test patterns for future contributors
- Update this document as issues are resolved

---

**Last Updated:** 2025-12-13
**Status:** Tracking active, issues pending creation
