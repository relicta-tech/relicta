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

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Release
        uses: relicta-tech/relicta-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
EOF
pause 4

# Save workflow
cat > .github/workflows/release.yml << 'EOF'
name: Release

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Release
        uses: relicta-tech/relicta-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
EOF

# Step 3: Explain key parts
echo ""
echo -e "${MAGENTA}━━━ Key Configuration Points ━━━${NC}"
echo ""
echo -e "  ${YELLOW}fetch-depth: 0${NC}     Fetch full history for commit analysis"
echo -e "  ${YELLOW}contents: write${NC}    Permission to create releases"
echo -e "  ${YELLOW}GITHUB_TOKEN${NC}       Built-in token for GitHub API"
pause 3

# Step 4: Advanced options
echo ""
echo -e "${MAGENTA}━━━ Advanced Options ━━━${NC}"
echo ""
cat << 'EOF'
      - name: Release
        uses: relicta-tech/relicta-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          command: full        # or: plan, bump, notes, publish
          version: latest      # or: v1.2.3
          auto-approve: true
          dry-run: false
EOF
pause 4

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} Automated releases on push to main"
echo -e "  ${GREEN}✓${NC} Uses official relicta-action"
echo -e "  ${GREEN}✓${NC} Creates GitHub releases automatically"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
