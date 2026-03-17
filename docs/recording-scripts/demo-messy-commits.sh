#!/bin/bash
# Demo: Messy Commits - Relicta handles any commit history

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create a temp repo with messy commits
DEMO_DIR="/tmp/relicta-messy-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Messy Commits? No Problem.${NC}"
echo -e "${CYAN}  Relicta handles any commit history${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Initialize git repo
git init -q
echo "# My Project" > README.md
git add . && git commit -q -m "initial"
git tag -a v1.0.0 -m "v1.0.0"

# Create messy commits
echo "function login() {}" > auth.js
git add . && git commit -q -m "wip"

echo "function logout() {}" >> auth.js
git add . && git commit -q -m "fixed the thing"

echo "// updated" >> auth.js
git add . && git commit -q -m "Update auth.js"

echo "const API_V2 = true" > api.js
git add . && git commit -q -m "BREAKING: new api!!!"

echo "function helper() {}" >> api.js
git add . && git commit -q -m "actually works now lol"

# Create config
cat > .relicta.yaml << 'EOF'
versioning:
  strategy: conventional
ai:
  enabled: false
EOF
git add . && git commit -q -m "chore: add config"

# Show the messy commits
echo -e "${BLUE}Commit history (real-world messy):${NC}"
echo ""
git log --oneline v1.0.0..HEAD
pause 3

# Run relicta plan
echo ""
echo -e "${GREEN}━━━ Running: relicta plan ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan${NC}"
pause 1
relicta plan
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Relicta detected:"
echo ""
echo -e "  ${GREEN}✓${NC} BREAKING keyword → Major version bump"
echo -e "  ${GREEN}✓${NC} 'fixed' keyword → Bug fix detected"
echo -e "  ${GREEN}✓${NC} Unclear commits → Grouped appropriately"
echo ""
echo -e "  ${MAGENTA}Pro tip:${NC} Use ${YELLOW}relicta notes --ai${NC} for best results!"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
