#!/bin/bash
# relicta publish - Success and Error cases

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
echo -e "${CYAN}  relicta publish - Execute release and run plugins${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup: complete workflow up to approval
echo -e "${YELLOW}Prerequisite: Complete plan → bump → notes → approve${NC}"
echo ""
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
relicta approve -y >/dev/null 2>&1
echo -e "${GREEN}✓ Release approved and ready to publish${NC}"
pause 2

# SUCCESS CASE 1: Basic publish
echo ""
echo -e "${GREEN}━━━ Success: Publish release ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta publish${NC}"
pause 0.5
relicta publish
pause 3

# Reset state (delete tag)
git tag -d v2.0.0 2>/dev/null || true

# SUCCESS CASE 2: Dry run
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
relicta approve -y >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Dry run (preview) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta publish --dry-run${NC}"
pause 0.5
relicta publish --dry-run
pause 3

# SUCCESS CASE 3: Skip push (local only)
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
relicta approve -y >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Skip push to remote ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta publish -P${NC}"
pause 0.5
relicta publish -P
pause 3

# Reset state
git tag -d v2.0.0 2>/dev/null || true

# SUCCESS CASE 4: Skip tag creation
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
relicta approve -y >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Skip tag creation ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta publish -T${NC}"
pause 0.5
relicta publish -T
pause 3

# ERROR CASE 1: Not approved
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
echo ""
echo -e "${RED}━━━ Error: Release not approved ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta publish${NC}"
pause 0.5
relicta publish 2>&1 || true
pause 2
echo ""
echo "  Fix: Run 'relicta approve' first"
pause 3

# ERROR CASE 2: No release in progress
relicta cancel -f 2>/dev/null || true
echo ""
echo -e "${RED}━━━ Error: No release in progress ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta publish${NC}"
pause 0.5
relicta publish 2>&1 || true
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -P, --skip-push   Skip pushing tags to remote"
echo "    -T, --skip-tag    Skip git tag creation"
echo "    --dry-run         Preview without changes"
echo "    --json            Output as JSON"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
git tag -d v2.0.0 2>/dev/null || true
