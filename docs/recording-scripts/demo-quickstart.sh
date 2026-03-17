#!/bin/bash
# Demo: Quick Start - Install to First Release
# Shows real CLI output from relicta commands

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create temp repo
DEMO_DIR="/tmp/relicta-quickstart-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Quick Start: Install to First Release${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Show version (simulates post-install)
echo -e "${YELLOW}Step 1: Verify Installation${NC}"
echo ""
echo -e "$ ${YELLOW}relicta version${NC}"
pause 1
relicta version
pause 2

# Initialize repo
echo ""
echo -e "${YELLOW}Step 2: Initialize Project${NC}"
echo ""
git init -q
echo "# My App" > README.md
git add . && git commit -q -m "initial commit"
git tag -a v1.0.0 -m "v1.0.0"

# Add some commits
echo "export function hello() { return 'Hello'; }" > app.js
git add . && git commit -q -m "feat: add hello function"

echo "export function goodbye() { return 'Goodbye'; }" >> app.js
git add . && git commit -q -m "feat: add goodbye function"

echo -e "$ ${YELLOW}relicta init --interactive=false${NC}"
pause 1
relicta init --force --interactive=false
pause 2

# Show release (dry-run to avoid actual publishing)
echo ""
echo -e "${YELLOW}Step 3: First Release${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release --yes --skip-push${NC}"
pause 1
relicta release --yes --skip-push
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} Installed with Homebrew"
echo -e "  ${GREEN}✓${NC} Initialized project with relicta init"
echo -e "  ${GREEN}✓${NC} Released v1.1.0 with one command"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
