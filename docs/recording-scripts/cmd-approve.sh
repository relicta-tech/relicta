#!/bin/bash
# relicta approve - Success and Error cases

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
echo -e "${CYAN}  relicta approve - Governance gate with audit trail${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup: plan, bump, and notes first
echo -e "${YELLOW}Prerequisite: Plan, bump, and notes first${NC}"
echo ""
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
echo -e "${GREEN}✓ Release planned, versioned, and notes generated${NC}"
pause 2

# SUCCESS CASE 1: Interactive approval
echo ""
echo -e "${GREEN}━━━ Success: Interactive approval ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta approve${NC}"
pause 0.5
echo "  (In interactive mode, user reviews and confirms)"
echo ""
# Simulate the approval prompt
echo "  📋 Release Summary"
echo "  ────────────────────────"
echo "  Version:     v2.0.0"
echo "  Commits:     4"
echo "  Risk Score:  0.42 (low)"
echo ""
echo "  Approve release? [y/N]: y"
echo ""
echo "  ✓ Release approved"
pause 3

# Reset for next demo
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1

# SUCCESS CASE 2: Auto-approve (CI/CD mode)
echo ""
echo -e "${GREEN}━━━ Success: Auto-approve (CI/CD mode) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta approve -y${NC}"
pause 0.5
relicta approve -y
pause 3

# Reset for next demo
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1

# SUCCESS CASE 3: Dry run
echo ""
echo -e "${GREEN}━━━ Success: Dry run ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta approve --dry-run${NC}"
pause 0.5
relicta approve --dry-run
pause 3

# ERROR CASE 1: No notes generated
relicta cancel -f 2>/dev/null || true
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
echo ""
echo -e "${RED}━━━ Error: Notes not generated ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta approve${NC}"
pause 0.5
relicta approve 2>&1 || true
pause 2
echo ""
echo "  Fix: Run 'relicta notes' first"
pause 3

# ERROR CASE 2: No release in progress
relicta cancel -f 2>/dev/null || true
echo ""
echo -e "${RED}━━━ Error: No release in progress ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta approve${NC}"
pause 0.5
relicta approve 2>&1 || true
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -y, --yes         Auto-approve without prompting"
echo "    --dry-run         Preview without changes"
echo "    --json            Output as JSON"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
