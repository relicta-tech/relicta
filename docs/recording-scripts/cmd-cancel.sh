#!/bin/bash
# relicta cancel - Success and Error cases

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
echo -e "${CYAN}  relicta cancel - Abort release in progress${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# SUCCESS CASE 1: Cancel planned release
relicta plan >/dev/null 2>&1
echo -e "${GREEN}━━━ Success: Cancel planned release ━━━${NC}"
echo ""
echo "Current state: planned"
echo ""
echo -e "$ ${YELLOW}relicta cancel -f${NC}"
pause 0.5
relicta cancel -f
pause 3

# SUCCESS CASE 2: Cancel after bump
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Cancel after version bump ━━━${NC}"
echo ""
echo "Current state: bumped (v2.0.0)"
echo ""
echo -e "$ ${YELLOW}relicta cancel -f${NC}"
pause 0.5
relicta cancel -f
pause 3

# SUCCESS CASE 3: Cancel after notes
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Cancel after notes generated ━━━${NC}"
echo ""
echo "Current state: noted"
echo ""
echo -e "$ ${YELLOW}relicta cancel -f${NC}"
pause 0.5
relicta cancel -f
pause 3

# SUCCESS CASE 4: Cancel approved release
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
relicta approve -y >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Cancel approved release ━━━${NC}"
echo ""
echo "Current state: approved"
echo ""
echo -e "$ ${YELLOW}relicta cancel -f${NC}"
pause 0.5
relicta cancel -f
pause 3

# ERROR CASE 1: No release in progress
echo ""
echo -e "${RED}━━━ Error: No release to cancel ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta cancel${NC}"
pause 0.5
relicta cancel 2>&1 || true
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -f, --force       Force cancel without confirmation"
echo ""
echo "  Use Cases:"
echo "    • Wrong commits analyzed"
echo "    • Need to include more changes"
echo "    • Version decision changed"
echo "    • Starting over from scratch"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
