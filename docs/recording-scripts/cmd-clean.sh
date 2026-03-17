#!/bin/bash
# relicta clean - Success and Error cases

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
echo -e "${CYAN}  relicta clean - Remove release state and artifacts${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup: create some state
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1

# Show current state
echo -e "${YELLOW}Current state before clean:${NC}"
echo ""
ls -la .relicta/ 2>/dev/null || echo "(no .relicta directory)"
pause 2

# SUCCESS CASE 1: Clean all state
echo ""
echo -e "${GREEN}━━━ Success: Clean all release state ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta clean -f${NC}"
pause 0.5
relicta clean -f
pause 2
echo ""
echo "After clean:"
ls -la .relicta/ 2>/dev/null || echo "(no .relicta directory - cleaned!)"
pause 3

# Setup again
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1

# SUCCESS CASE 2: Dry run
echo ""
echo -e "${GREEN}━━━ Success: Dry run (preview) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta clean --dry-run${NC}"
pause 0.5
relicta clean --dry-run
pause 3

# SUCCESS CASE 3: Clean specific artifacts
echo ""
echo -e "${GREEN}━━━ Success: Clean and show what was removed ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta clean -f -v${NC}"
pause 0.5
relicta clean -f -v
pause 3

# ERROR CASE 1: Nothing to clean
echo ""
echo -e "${RED}━━━ Info: Nothing to clean ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta clean${NC}"
pause 0.5
relicta clean -f 2>&1 || true
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -f, --force       Force clean without confirmation"
echo "    -v, --verbose     Show what is being removed"
echo "    --dry-run         Preview without changes"
echo ""
echo "  What gets cleaned:"
echo "    • .relicta/ directory (release state)"
echo "    • Temporary files"
echo "    • Cached plugin data"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
