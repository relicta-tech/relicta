#!/bin/bash
# relicta release - One-command release workflow

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Ensure clean state
relicta cancel -f 2>/dev/null || true

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  relicta release - Complete workflow in one command${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Runs: plan → bump → notes → approve → publish"
echo ""
pause 3

# SUCCESS CASE 1: Dry run (safe preview)
echo -e "${GREEN}━━━ Success: Dry run (preview entire workflow) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release --dry-run${NC}"
pause 0.5
relicta release --dry-run
pause 3

# SUCCESS CASE 2: Auto-approve (CI/CD mode)
echo ""
echo -e "${GREEN}━━━ Success: Auto-approve for CI/CD ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release -y${NC}"
pause 0.5
relicta release -y
pause 3

# Reset state (delete tag, reset)
git tag -d v2.0.0 2>/dev/null || true

# SUCCESS CASE 3: With AI notes
echo ""
echo -e "${GREEN}━━━ Success: With AI-powered notes ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release -y -a${NC}"
pause 0.5
relicta release -y -a
pause 3

# Reset state
git tag -d v2.0.0 2>/dev/null || true

# SUCCESS CASE 4: Force specific version
echo ""
echo -e "${GREEN}━━━ Success: Force specific version ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release -y -F v5.0.0${NC}"
pause 0.5
relicta release -y -F v5.0.0
pause 3

# Reset state
git tag -d v5.0.0 2>/dev/null || true

# SUCCESS CASE 5: Skip push (local only)
echo ""
echo -e "${GREEN}━━━ Success: Skip push to remote ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release -y -P${NC}"
pause 0.5
relicta release -y -P
pause 3

# ERROR CASE 1: No commits to release
git tag -d v2.0.0 2>/dev/null || true
git tag v1.0.1 2>/dev/null || true  # Tag current HEAD
echo ""
echo -e "${RED}━━━ Error: No commits since last release ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release${NC}"
pause 0.5
relicta release 2>&1 || true
git tag -d v1.0.1 2>/dev/null || true
pause 3

# ERROR CASE 2: Release already in progress
relicta plan >/dev/null 2>&1
echo ""
echo -e "${RED}━━━ Error: Release already in progress ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release${NC}"
pause 0.5
relicta release 2>&1 || true
pause 2
echo ""
echo "  Fix: Run 'relicta cancel' first, then retry"
pause 3

# Summary
relicta cancel -f 2>/dev/null || true
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -y, --yes         Auto-approve (skip confirmation)"
echo "    -a, --ai          Use AI for release notes"
echo "    -F, --force       Force specific version"
echo "    -P, --skip-push   Skip pushing to remote"
echo "    -T, --skip-tag    Skip git tag creation"
echo "    --dry-run         Preview entire workflow"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
