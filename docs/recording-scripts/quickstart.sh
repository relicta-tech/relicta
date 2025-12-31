#!/bin/bash
# Relicta Quick Start - Complete release workflow demo

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

type_slow() {
    echo -n "$1" | while IFS= read -r -n1 char; do
        echo -n "$char"
        sleep 0.02
    done
    echo ""
}

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Relicta Quick Start${NC}"
echo -e "${CYAN}  Governance for Software Change${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Step 1: Install
echo -e "${GREEN}Step 1: Install Relicta${NC}"
echo ""
echo -e "$ ${YELLOW}brew install relicta-tech/tap/relicta${NC}"
pause 0.5
echo "==> Installing relicta"
echo "🍺 relicta 2.9.0 installed"
pause 2

# Step 2: Initialize
echo ""
echo -e "${GREEN}Step 2: Initialize in your project${NC}"
echo ""
echo -e "$ ${YELLOW}cd my-project && relicta init${NC}"
pause 0.5
echo ""
echo "  ✓ Created .relicta.yaml"
echo "  ✓ Detected git repository"
echo "  ✓ Ready for releases"
pause 2

# Step 3: Check status
echo ""
echo -e "${GREEN}Step 3: Check project status${NC}"
echo ""
echo -e "$ ${YELLOW}relicta status${NC}"
pause 0.5
relicta status
pause 2

# Step 4: Plan release
echo ""
echo -e "${GREEN}Step 4: Plan your release${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan --analyze${NC}"
pause 0.5
relicta plan --analyze
pause 2

# Step 5: Set version
echo ""
echo -e "${GREEN}Step 5: Calculate version${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump${NC}"
pause 0.5
relicta bump
pause 2

# Step 6: Generate notes
echo ""
echo -e "${GREEN}Step 6: Generate release notes${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 0.5
relicta notes
pause 2

# Step 7: Approve
echo ""
echo -e "${GREEN}Step 7: Approve the release${NC}"
echo ""
echo -e "$ ${YELLOW}relicta approve -y${NC}"
pause 0.5
relicta approve -y
pause 2

# Step 8: Publish
echo ""
echo -e "${GREEN}Step 8: Publish!${NC}"
echo ""
echo -e "$ ${YELLOW}relicta publish -P${NC}"
pause 0.5
relicta publish -P
pause 3

# Or use release command
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BLUE}  💡 Pro tip: Do it all in one command!${NC}"
echo ""
echo -e "  $ ${YELLOW}relicta release -y${NC}"
echo ""
echo "  This runs: plan → bump → notes → approve → publish"
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Summary
clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  ${GREEN}✨ You're ready to use Relicta!${NC}"
echo ""
echo "  Core Commands:"
echo "    relicta plan      Analyze commits"
echo "    relicta bump      Calculate version"
echo "    relicta notes     Generate release notes"
echo "    relicta approve   Governance gate"
echo "    relicta publish   Execute release"
echo "    relicta release   All of the above"
echo ""
echo "  Helpful Commands:"
echo "    relicta status    View release state"
echo "    relicta cancel    Abort release"
echo "    relicta policy    Manage governance"
echo ""
echo -e "  ${BLUE}Learn more: relicta.dev/docs${NC}"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
git tag -d v2.0.0 2>/dev/null || true
