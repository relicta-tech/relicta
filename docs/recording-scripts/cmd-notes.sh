#!/bin/bash
# relicta notes - Success and Error cases

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
echo -e "${CYAN}  relicta notes - Generate release notes${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup: plan and bump first
echo -e "${YELLOW}Prerequisite: Plan and bump first${NC}"
echo ""
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
echo -e "${GREEN}✓ Release planned and versioned${NC}"
pause 2

# SUCCESS CASE 1: Basic notes (no AI)
echo ""
echo -e "${GREEN}━━━ Success: Generate notes (template-based) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 0.5
relicta notes
pause 3

# Reset for next demo
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1

# SUCCESS CASE 2: Write a copy to a file
echo ""
echo -e "${GREEN}━━━ Success: Write notes to a file ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes --output /tmp/relicta-demo-notes.md${NC}"
pause 0.5
relicta notes --output /tmp/relicta-demo-notes.md
pause 2
echo ""
echo "Notes copy written:"
head -20 /tmp/relicta-demo-notes.md 2>/dev/null || echo "(No output file)"
pause 3

# SUCCESS CASE 3: Machine-readable output
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: JSON output ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes --json${NC}"
pause 0.5
relicta notes --json
pause 3

# ERROR CASE 1: No version set
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
echo ""
echo -e "${RED}━━━ Error: Version not set (bump not run) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 0.5
relicta notes 2>&1 || true
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    --output          Write a copy of the generated notes to a file"
echo "    --audience        AI audience: developers, users, public, stakeholders"
echo "    --tone            AI tone: technical, friendly, professional, marketing"
echo "    --language        Language for configured AI-generated notes"
echo "    --emoji           Include emojis in generated notes"
echo "    --json            Output note data as JSON"
echo ""
echo "  AI prose requires ai.enabled and a working provider in .relicta.yaml."
echo "  For product-context customer copy, use the MCP agent-session workflow."
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
rm -f /tmp/relicta-demo-notes.md
