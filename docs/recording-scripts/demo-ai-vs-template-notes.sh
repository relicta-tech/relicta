#!/bin/bash
# Demo: AI vs Template Release Notes - Quality Comparison
#
# This demo shows the dramatic difference between:
# 1. Template-based release notes (basic categorization)
# 2. AI-powered release notes (contextual, user-friendly)

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  AI vs Template Release Notes${NC}"
echo -e "${CYAN}  See the quality difference${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Ensure clean state
relicta cancel -f 2>/dev/null || true

# Setup
echo -e "${YELLOW}Setting up release...${NC}"
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
echo -e "${GREEN}✓ Ready to generate notes${NC}"
echo ""
pause 2

# Show the commits we're working with
echo -e "${BLUE}━━━ Commits being analyzed ━━━${NC}"
echo ""
git log --oneline v1.0.0..HEAD 2>/dev/null || git log --oneline -5
echo ""
pause 3

# Template-based notes (default)
echo ""
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${MAGENTA}  TEMPLATE-BASED NOTES (default)${NC}"
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 1
relicta notes
pause 4

# Reset for AI notes
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1

# AI-powered notes
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  AI-POWERED NOTES${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes --ai${NC}"
pause 1
relicta notes --ai
pause 4

# Comparison summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Key Differences:"
echo ""
echo -e "  ${MAGENTA}Template Notes:${NC}"
echo "    • Lists commit messages verbatim"
echo "    • Groups by conventional commit type"
echo "    • Technical, developer-focused"
echo "    • No context or explanation"
echo ""
echo -e "  ${GREEN}AI-Powered Notes:${NC}"
echo "    • Summarizes changes meaningfully"
echo "    • Highlights user impact"
echo "    • Professional, user-friendly tone"
echo "    • Explains breaking changes"
echo "    • Adds migration guidance"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  Enable AI notes: ${YELLOW}relicta notes --ai${NC}"
echo -e "  Or in CI/CD:     ${YELLOW}relicta release -y --ai${NC}"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 4

# Cleanup
relicta cancel -f 2>/dev/null || true
