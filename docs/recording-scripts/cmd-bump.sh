#!/bin/bash
# relicta bump - Success and Error cases

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  relicta bump - Calculate and apply version bump${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# First, ensure we have a planned release
echo -e "${YELLOW}Prerequisite: Run 'relicta plan' first${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan${NC}"
relicta plan
pause 2

# SUCCESS CASE 1: Auto bump
echo ""
echo -e "${GREEN}━━━ Success: Auto bump (from commits) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump${NC}"
pause 0.5
relicta bump
pause 3

# Reset for next demo
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1

# SUCCESS CASE 2: Force specific bump level
echo ""
echo -e "${GREEN}━━━ Success: Force minor bump ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump -l minor${NC}"
pause 0.5
relicta bump -l minor
pause 3

# Reset for next demo
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1

# SUCCESS CASE 3: Force specific version
echo ""
echo -e "${GREEN}━━━ Success: Force specific version ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump -F v3.0.0${NC}"
pause 0.5
relicta bump -F v3.0.0
pause 3

# Reset for next demo
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1

# SUCCESS CASE 4: Dry run
echo ""
echo -e "${GREEN}━━━ Success: Dry run ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump --dry-run${NC}"
pause 0.5
relicta bump --dry-run
pause 3

# ERROR CASE 1: No plan exists
relicta cancel -f 2>/dev/null || true
echo ""
echo -e "${RED}━━━ Error: No release planned ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump${NC}"
pause 0.5
relicta bump 2>&1 || true
pause 3

# ERROR CASE 2: Invalid version format
relicta plan >/dev/null 2>&1
echo ""
echo -e "${RED}━━━ Error: Invalid version format ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump -F invalid${NC}"
pause 0.5
relicta bump -F invalid 2>&1 || true
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -l, --level       Bump type: major, minor, patch, auto"
echo "    -F, --force       Force specific version (e.g., v2.0.0)"
echo "    --dry-run         Preview without changes"
echo "    --json            Output as JSON"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
