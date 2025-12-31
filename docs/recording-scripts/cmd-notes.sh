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

# SUCCESS CASE 2: AI-powered notes
echo ""
echo -e "${GREEN}━━━ Success: AI-powered notes ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes -a${NC}"
pause 0.5
relicta notes -a
pause 3

# Reset for next demo
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1

# SUCCESS CASE 3: Save to changelog
echo ""
echo -e "${GREEN}━━━ Success: Save notes to changelog ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes -s${NC}"
pause 0.5
relicta notes -s
pause 2
echo ""
echo "Changelog updated:"
head -20 CHANGELOG.md 2>/dev/null || echo "(No changelog file)"
pause 3

# SUCCESS CASE 4: Dry run
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Dry run ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes --dry-run${NC}"
pause 0.5
relicta notes --dry-run
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

# ERROR CASE 2: AI without API key (simulated)
echo ""
echo -e "${RED}━━━ Error: AI requested but no API key ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}OPENAI_API_KEY= relicta notes -a${NC}"
pause 0.5
echo "✗ AI provider not configured"
echo "  Set OPENAI_API_KEY or configure ai.provider in .relicta.yaml"
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -a, --ai          Use AI to generate notes"
echo "    -s, --save        Save to changelog file"
echo "    --tone            AI tone: professional, casual, technical"
echo "    --audience        Target: developers, users, stakeholders"
echo "    --dry-run         Preview without changes"
echo "    --json            Output as JSON"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
