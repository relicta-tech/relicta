#!/bin/bash
# Marketing demo - showcases full workflow

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Relicta${NC}"
echo -e "${CYAN}  The governance layer for software change${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Show commits
echo -e "${BLUE}Commits since last release:${NC}"
echo ""
git log --oneline v1.0.0..HEAD | head -4
pause 2

# Run release
echo ""
echo -e "${GREEN}Release with one command:${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release --yes --skip-push${NC}"
pause 1
relicta release --yes --skip-push
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  ${GREEN}✓ Semantic versioning${NC}"
echo -e "  ${GREEN}✓ Release notes generation${NC}"
echo -e "  ${GREEN}✓ Governance & approval${NC}"
echo -e "  ${GREEN}✓ Multi-platform publishing${NC}"
echo ""
echo -e "  ${MAGENTA}brew install relicta-tech/tap/relicta${NC}"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
git tag -d v2.0.0 2>/dev/null || true
