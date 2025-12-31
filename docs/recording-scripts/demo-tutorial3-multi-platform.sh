#!/bin/bash
# Demo: Tutorial 3 - Multi-Platform Publishing
# Shows installing multiple plugins and coordinated release

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Create temp repo
DEMO_DIR="/tmp/relicta-multi-platform-demo"
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Multi-Platform Publishing${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup minimal repo
git init -q
echo "# Multi-Platform Demo" > README.md
git add . && git commit -q -m "initial"

# Step 1: Install multiple plugins
echo -e "${MAGENTA}━━━ Step 1: Install Multiple Plugins ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plugin install github${NC}"
pause 1
relicta plugin install github 2>/dev/null || echo -e "${GREEN}✓ GitHub plugin already installed${NC}"
pause 2

echo ""
echo -e "$ ${YELLOW}relicta plugin install slack${NC}"
pause 1
relicta plugin install slack 2>/dev/null || echo -e "${GREEN}✓ Slack plugin already installed${NC}"
pause 2

echo ""
echo -e "$ ${YELLOW}relicta plugin install discord${NC}"
pause 1
relicta plugin install discord 2>/dev/null || echo -e "${GREEN}✓ Discord plugin already installed${NC}"
pause 2

# Step 2: List installed plugins
echo ""
echo -e "${MAGENTA}━━━ Step 2: Verify Installed Plugins ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plugin list${NC}"
pause 1
relicta plugin list
pause 3

# Step 3: Show multi-platform config
echo ""
echo -e "${MAGENTA}━━━ Step 3: Multi-Platform Configuration ━━━${NC}"
echo ""
echo -e "${CYAN}release.config.yaml:${NC}"
echo ""
cat << 'EOF'
plugins:
  - name: github
    enabled: true
    config:
      create_release: true
      assets: ["dist/*.tar.gz"]

  - name: slack
    enabled: true
    config:
      webhook_url: ${SLACK_WEBHOOK}
      channel: "#releases"

  - name: discord
    enabled: true
    config:
      webhook_url: ${DISCORD_WEBHOOK}
EOF
pause 4

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} GitHub  - Create releases with assets"
echo -e "  ${GREEN}✓${NC} Slack   - Notify team channel"
echo -e "  ${GREEN}✓${NC} Discord - Notify community"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  One ${YELLOW}relicta publish${NC} runs all plugins automatically!"
pause 3

# Cleanup
rm -rf "$DEMO_DIR"
