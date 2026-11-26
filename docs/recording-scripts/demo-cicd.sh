#!/bin/bash
# Demo: CI/CD Integration
# Shows setting up GitHub Actions for automated releases

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create temp directory
DEMO_DIR="/tmp/relicta-cicd-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  CI/CD Integration with GitHub Actions${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup repo
git init -q
echo "# CI/CD Demo" > README.md
git add . && git commit -q -m "initial"

# Step 1: Create workflow directory
echo -e "${MAGENTA}━━━ Step 1: Create Workflow Directory ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}mkdir -p .github/workflows${NC}"
mkdir -p .github/workflows
echo -e "${GREEN}✓ Created .github/workflows/${NC}"
pause 2

# Step 2: Create release workflow
echo ""
echo -e "${MAGENTA}━━━ Step 2: Create Release Workflow ━━━${NC}"
echo ""
echo -e "${CYAN}.github/workflows/release.yml:${NC}"
echo ""

cat << 'EOF'
name: Release

on: workflow_dispatch

permissions:
  contents: read

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v6
        with:
          fetch-depth: 0
          persist-credentials: false

      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v6
        with:
          go-version: '1.26'
      - run: go install github.com/relicta-tech/relicta/v4/cmd/relicta@v4.2.0
      - run: relicta plan --dry-run --json
EOF
pause 4

# Save workflow
cat > .github/workflows/release.yml << 'EOF'
name: Release

on: workflow_dispatch

permissions:
  contents: read

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v6
        with:
          fetch-depth: 0
          persist-credentials: false

      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v6
        with:
          go-version: '1.26'
      - run: go install github.com/relicta-tech/relicta/v4/cmd/relicta@v4.2.0
      - run: relicta plan --dry-run --json
EOF

# Step 3: Explain key parts
echo ""
echo -e "${MAGENTA}━━━ Key Configuration Points ━━━${NC}"
echo ""
echo -e "  ${YELLOW}fetch-depth: 0${NC}     Fetch full history for commit analysis"
echo -e "  ${YELLOW}contents: read${NC}     Minimum permission for a release plan"
echo -e "  ${YELLOW}Go module install${NC}  Installs a pinned Relicta version"
pause 3

# Step 4: Advanced options
echo ""
echo -e "${MAGENTA}━━━ Advanced Options ━━━${NC}"
echo ""
cat << 'EOF'
      - name: Release
        run: relicta plan --dry-run --json
EOF
pause 4

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} Runs a read-only release plan on demand"
echo -e "  ${GREEN}✓${NC} Uses a pinned Relicta module version"
echo -e "  ${GREEN}✓${NC} Requires no publishing credentials"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
