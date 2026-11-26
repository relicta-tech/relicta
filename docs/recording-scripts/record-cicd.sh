#!/bin/bash
# CI/CD Integration demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
DIM='\033[2m'
NC='\033[0m'

pause() { sleep "${1:-1}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  CI/CD Integration${NC}"
echo -e "${CYAN}  Automate releases with GitHub Actions${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

echo -e "${GREEN}Create the workflow file:${NC}"
echo -e "${DIM}.github/workflows/release.yml${NC}"
echo ""
pause 1

cat << 'EOF'
name: Release

on:
  push:
    branches: [main]

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

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${GREEN}What happens when this workflow runs:${NC}"
echo ""
echo "  1. Analyzes commits since last release"
echo "  2. Calculates semantic version"
echo "  3. Prints the recommendation as JSON"
echo ""
echo -e "${BLUE}Pinned CLI install. Read-only workflow. No publishing credentials.${NC}"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 4
