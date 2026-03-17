#!/bin/bash
# One-command release demo

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

pause() { sleep "${1:-1}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  One-Command Release${NC}"
echo -e "${CYAN}  Complete workflow in one step${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Show commits
echo -e "${BLUE}Commits since last release:${NC}"
git log --oneline v1.0.0..HEAD | head -5
echo ""
pause 2

# Run the release
echo -e "${GREEN}Run the complete workflow:${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release --yes --skip-push${NC}"
pause 1

relicta release --yes --skip-push

pause 3

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  The 'relicta release' command runs:"
echo ""
echo "    1. plan    - Analyze commits"
echo "    2. bump    - Calculate version"
echo "    3. notes   - Generate changelog"
echo "    4. approve - Governance gate"
echo "    5. publish - Create release"
echo ""
echo -e "  Flags: ${YELLOW}--yes${NC} (auto-approve) ${YELLOW}--skip-push${NC} (local only)"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
git tag -d v2.0.0 2>/dev/null || true
