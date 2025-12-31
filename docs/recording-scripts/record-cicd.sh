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
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: relicta-tech/relicta-action@v1
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
EOF

pause 4

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${GREEN}What happens on each push to main:${NC}"
echo ""
echo "  1. Analyzes commits since last release"
echo "  2. Calculates semantic version"
echo "  3. Generates professional release notes"
echo "  4. Creates GitHub Release with changelog"
echo "  5. Tags the commit"
echo ""
echo -e "${BLUE}Zero configuration. Zero install. Just works.${NC}"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 4
