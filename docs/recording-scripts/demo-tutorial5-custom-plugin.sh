#!/bin/bash
# Demo: Tutorial 5 - Custom Plugin Development
# Shows real CLI output for plugin create, build, install

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create temp directory
DEMO_DIR="/tmp/relicta-custom-plugin-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Custom Plugin Development${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Step 1: Create plugin
echo -e "${MAGENTA}━━━ Step 1: Create Plugin Project ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plugin create my-notifier${NC}"
pause 1
relicta plugin create my-notifier
pause 3

# Step 2: Show plugin structure
echo ""
echo -e "${MAGENTA}━━━ Step 2: Plugin Structure ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}ls -la my-notifier/${NC}"
pause 1
ls -la my-notifier/
pause 2

echo ""
echo -e "$ ${YELLOW}cat my-notifier/main.go | head -40${NC}"
pause 1
cat my-notifier/main.go | head -40
pause 3

# Step 3: Build instructions
echo ""
echo -e "${MAGENTA}━━━ Step 3: Build & Install ━━━${NC}"
echo ""
echo -e "${CYAN}To build and install your plugin:${NC}"
echo ""
cat << 'STEPS'
  1. cd my-notifier
  2. Update go.mod module path
  3. go mod tidy
  4. go build -o my-notifier .
  5. relicta plugin install --local ./my-notifier
STEPS
pause 4

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} Created plugin scaffold with relicta plugin create"
echo -e "  ${GREEN}✓${NC} Plugin includes main.go with hook implementations"
echo -e "  ${GREEN}✓${NC} Uses standard Go toolchain to build"
echo -e "  ${GREEN}✓${NC} Install locally with relicta plugin install"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
cd /tmp
rm -rf "$DEMO_DIR"
