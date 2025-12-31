#!/bin/bash
# Demo: Tutorial 1 - Basic Release Workflow
# Shows real CLI output for plan, bump, notes, approve, publish

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create temp repo
DEMO_DIR="/tmp/relicta-basic-workflow-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Basic Release Workflow: Step by Step${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup repo
git init -q
echo "# Basic Workflow Demo" > README.md
git add . && git commit -q -m "initial"
git tag -a v1.0.0 -m "v1.0.0"

# Add feature commits
echo "export const api = { get: () => {} };" > api.js
git add . && git commit -q -m "feat(api): add GET endpoint"

echo "export const api = { get: () => {}, post: () => {} };" > api.js
git add . && git commit -q -m "feat(api): add POST endpoint"

echo "// Fixed null check" >> api.js
git add . && git commit -q -m "fix(api): handle null response"

# Minimal config
cat > .relicta.yaml << 'EOF'
versioning:
  strategy: conventional
ai:
  enabled: false
EOF
git add . && git commit -q -m "chore: add config"

# Show commits
echo -e "${YELLOW}Commits since v1.0.0:${NC}"
git log --oneline v1.0.0..HEAD
pause 2

# Step 1: Plan
echo ""
echo -e "${MAGENTA}━━━ Step 1: Analyze Changes ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan${NC}"
pause 1
relicta plan
pause 3

# Step 2: Bump
echo ""
echo -e "${MAGENTA}━━━ Step 2: Bump Version ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump${NC}"
pause 1
relicta bump
pause 3

# Step 3: Notes
echo ""
echo -e "${MAGENTA}━━━ Step 3: Generate Release Notes ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 1
relicta notes
pause 3

# Step 4: Approve
echo ""
echo -e "${MAGENTA}━━━ Step 4: Approve Release ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta approve --yes${NC}"
pause 1
relicta approve --yes
pause 3

# Step 5: Publish
echo ""
echo -e "${MAGENTA}━━━ Step 5: Publish Release ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta publish --skip-push${NC}"
pause 1
relicta publish --skip-push
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} plan    - Analyzed 4 commits"
echo -e "  ${GREEN}✓${NC} bump    - Set version to v1.1.0"
echo -e "  ${GREEN}✓${NC} notes   - Generated changelog"
echo -e "  ${GREEN}✓${NC} approve - Approved for release"
echo -e "  ${GREEN}✓${NC} publish - Created git tag"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
