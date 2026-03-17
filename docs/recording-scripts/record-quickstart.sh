#!/bin/bash
# Simplified quickstart recording script

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pause() { sleep "${1:-1}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Relicta Quick Start${NC}"
echo -e "${CYAN}  From install to first release${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Simulate install
echo -e "${GREEN}Step 1: Install${NC}"
echo -e "$ ${YELLOW}brew install relicta-tech/tap/relicta${NC}"
pause 1
echo "==> Installing relicta"
echo "🍺 relicta installed"
pause 2

# Show init
echo ""
echo -e "${GREEN}Step 2: Initialize${NC}"
echo -e "$ ${YELLOW}relicta init${NC}"
pause 1
echo "✓ Created .relicta.yaml"
pause 2

# Run actual release
echo ""
echo -e "${GREEN}Step 3: Release!${NC}"
echo -e "$ ${YELLOW}relicta release --yes --skip-push${NC}"
pause 1

# Cancel any existing release first
relicta cancel -f 2>/dev/null || true

# Run the release
relicta release --yes --skip-push

pause 3

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓ Released v2.0.0 in seconds!${NC}"
echo ""
echo "  Get started: brew install relicta-tech/tap/relicta"
echo "  Learn more:  docs.relicta.tech"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
git tag -d v2.0.0 2>/dev/null || true
