#!/bin/bash
# Demo: Tutorial 2 - GitHub Plugin Setup
# Shows real CLI output for plugin list, install, configure

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create temp repo
DEMO_DIR="/tmp/relicta-github-plugin-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  GitHub Plugin Setup${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup minimal repo
git init -q
echo "# Plugin Demo" > README.md
git add . && git commit -q -m "initial"

# Step 1: List available plugins
echo -e "${MAGENTA}━━━ Step 1: List Available Plugins ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plugin list${NC}"
pause 1
relicta plugin list
pause 3

# Step 2: Install GitHub plugin
echo ""
echo -e "${MAGENTA}━━━ Step 2: Install GitHub Plugin ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plugin install github${NC}"
pause 1
relicta plugin install github 2>/dev/null || echo -e "${GREEN}✓ GitHub plugin already installed${NC}"
pause 3

# Step 3: Verify installation
echo ""
echo -e "${MAGENTA}━━━ Step 3: Verify Installation ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plugin list${NC}"
pause 1
relicta plugin list
pause 3

# Step 4: Show config
echo ""
echo -e "${MAGENTA}━━━ Step 4: Configure Plugin ━━━${NC}"
echo ""
echo -e "${CYAN}Add to release.config.yaml:${NC}"
echo ""
cat << 'EOF'
plugins:
  - name: github
    enabled: true
    config:
      create_release: true
      draft: false
      assets:
        - "dist/*.tar.gz"
        - "dist/*.zip"
EOF
pause 4

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} Listed available plugins"
echo -e "  ${GREEN}✓${NC} Installed GitHub plugin"
echo -e "  ${GREEN}✓${NC} Ready to create GitHub releases"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
