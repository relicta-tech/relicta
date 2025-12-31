#!/bin/bash
# relicta status - Success and Error cases

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
echo -e "${CYAN}  relicta status - View current release state${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# SUCCESS CASE 1: No release in progress (clean state)
relicta cancel -f 2>/dev/null || true
echo -e "${GREEN}━━━ Success: No release in progress ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta status${NC}"
pause 0.5
relicta status
pause 3

# SUCCESS CASE 2: After planning
relicta plan >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Release planned ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta status${NC}"
pause 0.5
relicta status
pause 3

# SUCCESS CASE 3: After bumping version
relicta bump >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Version set ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta status${NC}"
pause 0.5
relicta status
pause 3

# SUCCESS CASE 4: After notes generated
relicta notes >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Notes generated ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta status${NC}"
pause 0.5
relicta status
pause 3

# SUCCESS CASE 5: After approval
relicta approve -y >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Release approved ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta status${NC}"
pause 0.5
relicta status
pause 3

# SUCCESS CASE 6: JSON output
echo ""
echo -e "${GREEN}━━━ Success: JSON output ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta status --json${NC}"
pause 0.5
relicta status --json
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    --json            Output as JSON"
echo ""
echo "  States:"
echo "    draft      → No release in progress"
echo "    planned    → Commits analyzed, ready for version"
echo "    bumped     → Version calculated"
echo "    noted      → Release notes generated"
echo "    approved   → Ready to publish"
echo "    published  → Release complete"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
