#!/bin/bash
# Demo: Messy Commits - Real command output

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create temp repo with messy commits
DEMO_DIR="/tmp/relicta-messy-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Messy Commits? No Problem.${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Initialize repo
git init -q
echo "# Project" > README.md
git add . && git commit -q -m "initial"
git tag -a v1.0.0 -m "v1.0.0"

# Create messy commits (real-world style)
echo "login()" > auth.js
git add . && git commit -q -m "wip"

echo "logout()" >> auth.js
git add . && git commit -q -m "fixed the thing"

echo "// api v2" > api.js
git add . && git commit -q -m "BREAKING: new api!!!"

echo "helper()" >> api.js
git add . && git commit -q -m "actually works now lol"

# Create minimal config
cat > .relicta.yaml << 'EOF'
versioning:
  strategy: conventional
ai:
  enabled: false
EOF
git add . && git commit -q -m "config"

# Show messy history
echo -e "${YELLOW}Messy commit history:${NC}"
echo ""
git log --oneline v1.0.0..HEAD
pause 3

# Run relicta plan - show real output
echo ""
echo -e "$ ${YELLOW}relicta plan${NC}"
pause 1
relicta plan
pause 3

# Run bump - show version detection
echo ""
echo -e "$ ${YELLOW}relicta bump${NC}"
pause 1
relicta bump
pause 3

# Generate notes - show how it handles messy commits
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 1
relicta notes
pause 3

# Show the changelog
echo ""
echo -e "${CYAN}Generated CHANGELOG.md:${NC}"
cat CHANGELOG.md
pause 4

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} Detected BREAKING → Major bump"
echo -e "  ${GREEN}✓${NC} Detected 'fixed' → Bug fix"
echo -e "  ${GREEN}✓${NC} Grouped unclear commits appropriately"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
