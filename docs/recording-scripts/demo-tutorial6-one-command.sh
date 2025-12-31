#!/bin/bash
# Demo: Tutorial 6 - One-Command Release
# Shows real CLI output for relicta release with various flags

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create temp repo
DEMO_DIR="/tmp/relicta-one-command-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  One-Command Release (v2.6.0+)${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup repo
git init -q
echo "# One Command Demo" > README.md
git add . && git commit -q -m "initial"
git tag -a v1.0.0 -m "v1.0.0"

# Add several commits
echo "export const cache = {};" > cache.js
git add . && git commit -q -m "feat: add caching layer"

echo "export const db = { connect: () => {} };" > db.js
git add . && git commit -q -m "feat(db): add database connection"

echo "// performance fix" >> db.js
git add . && git commit -q -m "perf: optimize db queries"

echo "// fixed bug" >> cache.js
git add . && git commit -q -m "fix: resolve cache invalidation"

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

# Explain the command
echo ""
echo -e "${MAGENTA}━━━ The One-Command Workflow ━━━${NC}"
echo ""
echo -e "  ${CYAN}relicta release${NC} runs all 5 steps automatically:"
echo ""
echo -e "    1. ${YELLOW}plan${NC}    → Analyze commits"
echo -e "    2. ${YELLOW}bump${NC}    → Calculate version"
echo -e "    3. ${YELLOW}notes${NC}   → Generate changelog"
echo -e "    4. ${YELLOW}approve${NC} → Review & approve"
echo -e "    5. ${YELLOW}publish${NC} → Create tag & release"
pause 3

# Run the one-command release
echo ""
echo -e "${MAGENTA}━━━ Running relicta release ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta release --yes --skip-push${NC}"
pause 1
relicta release --yes --skip-push
pause 3

# Show key flags
echo ""
echo -e "${MAGENTA}━━━ Key Flags ━━━${NC}"
echo ""
echo -e "  ${YELLOW}--yes${NC}        Auto-approve without prompts"
echo -e "  ${YELLOW}--dry-run${NC}    Preview without making changes"
echo -e "  ${YELLOW}--force${NC}      Release even with existing state"
echo -e "  ${YELLOW}--skip-push${NC}  Don't push tags to remote"
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} Complete workflow in one command"
echo -e "  ${GREEN}✓${NC} Automatic version calculation"
echo -e "  ${GREEN}✓${NC} Release notes generated"
echo -e "  ${GREEN}✓${NC} Git tag created"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
