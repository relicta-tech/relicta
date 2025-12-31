#!/bin/bash
# relicta plan - Success and Error cases

set -e
cd /tmp/relicta-demo

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  relicta plan - Analyze commits and plan release${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# SUCCESS CASE 1: Basic plan
echo -e "${GREEN}━━━ Success: Basic plan ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan${NC}"
pause 0.5
relicta plan
pause 3

# SUCCESS CASE 2: Plan with analysis
echo ""
echo -e "${GREEN}━━━ Success: Plan with detailed analysis ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan -a${NC}"
pause 0.5
relicta plan -a
pause 3

# SUCCESS CASE 3: Plan from specific ref
echo ""
echo -e "${GREEN}━━━ Success: Plan from specific reference ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan -f v1.0.0 -t HEAD${NC}"
pause 0.5
relicta plan -f v1.0.0 -t HEAD
pause 3

# SUCCESS CASE 4: Dry run
echo ""
echo -e "${GREEN}━━━ Success: Dry run (preview only) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan --dry-run${NC}"
pause 0.5
relicta plan --dry-run
pause 3

# ERROR CASE 1: No commits since last tag
echo ""
echo -e "${RED}━━━ Error: No commits since last release ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan -f HEAD -t HEAD${NC}"
pause 0.5
relicta plan -f HEAD -t HEAD 2>&1 || true
pause 3

# ERROR CASE 2: Invalid reference
echo ""
echo -e "${RED}━━━ Error: Invalid git reference ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan -f nonexistent-tag${NC}"
pause 0.5
relicta plan -f nonexistent-tag 2>&1 || true
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -a, --analyze     Include detailed commit analysis"
echo "    -f, --from        Starting reference (default: latest tag)"
echo "    -t, --to          Ending reference (default: HEAD)"
echo "    --dry-run         Preview without changes"
echo "    --json            Output as JSON"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
