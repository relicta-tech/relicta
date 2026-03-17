#!/bin/bash
# Demo: AI vs Template Notes - Real command output comparison

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  AI vs Template Release Notes${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Clean state
relicta cancel -f 2>/dev/null || true

# Show commits
echo -e "${YELLOW}Commits since v1.0.0:${NC}"
git log --oneline v1.0.0..HEAD
pause 2

# Setup for template notes
echo ""
echo -e "${MAGENTA}━━━ Template Notes (default) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan${NC}"
pause 1
relicta plan
pause 2

echo ""
echo -e "$ ${YELLOW}relicta bump${NC}"
pause 1
relicta bump
pause 2

echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 1
relicta notes
pause 4

# Show the generated changelog
echo ""
echo -e "${CYAN}Generated CHANGELOG.md:${NC}"
cat CHANGELOG.md | head -25
pause 4

# Reset and do AI notes
relicta cancel -f 2>/dev/null || true

echo ""
echo -e "${GREEN}━━━ AI-Powered Notes ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan${NC}"
pause 1
relicta plan
pause 2

echo ""
echo -e "$ ${YELLOW}relicta bump${NC}"
pause 1
relicta bump
pause 2

echo ""
echo -e "$ ${YELLOW}relicta notes --ai${NC}"
pause 1
relicta notes --ai
pause 4

# Show the AI-generated changelog
echo ""
echo -e "${CYAN}Generated CHANGELOG.md (AI):${NC}"
cat CHANGELOG.md | head -30
pause 4

# Cleanup
relicta cancel -f 2>/dev/null || true
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} Template: Technical commit listing"
echo -e "  ${GREEN}✓${NC} AI: Professional, user-friendly summary"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
