#!/bin/bash
# relicta blast - Success and Error cases

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
echo -e "${CYAN}  relicta blast - Send release announcements${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Setup: complete a release first
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes >/dev/null 2>&1
relicta approve -y >/dev/null 2>&1
relicta publish -P >/dev/null 2>&1
echo -e "${GREEN}✓ Release v2.0.0 published (local)${NC}"
pause 2

# SUCCESS CASE 1: Blast to all configured channels
echo ""
echo -e "${GREEN}━━━ Success: Announce to all channels ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta blast${NC}"
pause 0.5
# Simulate blast output
echo ""
echo "  📢 Sending release announcements..."
echo ""
echo "  Channel: slack"
echo "    ✓ Message sent to #releases"
echo ""
echo "  Channel: discord"
echo "    ✓ Message sent to announcements"
echo ""
echo "  ✓ Announcements sent to 2 channels"
pause 3

# SUCCESS CASE 2: Blast to specific channel
echo ""
echo -e "${GREEN}━━━ Success: Announce to specific channel ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta blast -c slack${NC}"
pause 0.5
echo ""
echo "  📢 Sending release announcements..."
echo ""
echo "  Channel: slack"
echo "    ✓ Message sent to #releases"
echo ""
echo "  ✓ Announcements sent to 1 channel"
pause 3

# SUCCESS CASE 3: Dry run
echo ""
echo -e "${GREEN}━━━ Success: Dry run (preview) ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta blast --dry-run${NC}"
pause 0.5
echo ""
echo "  📢 [DRY RUN] Would send release announcements..."
echo ""
echo "  Channel: slack"
echo "    → Would send to #releases"
echo ""
echo "  Channel: discord"
echo "    → Would send to announcements"
echo ""
echo "  [DRY RUN] No announcements sent"
pause 3

# ERROR CASE 1: No release published
git tag -d v2.0.0 2>/dev/null || true
relicta cancel -f 2>/dev/null || true
echo ""
echo -e "${RED}━━━ Error: No release to announce ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta blast${NC}"
pause 0.5
echo ""
echo "  ✗ No recent release found"
echo "  Run 'relicta publish' first"
pause 3

# ERROR CASE 2: No announcement channels configured
echo ""
echo -e "${RED}━━━ Error: No channels configured ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta blast${NC}"
pause 0.5
echo ""
echo "  ✗ No announcement channels configured"
echo "  Add plugins with announcement support (slack, discord, etc.)"
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -c, --channel     Target specific channel (slack, discord, etc.)"
echo "    --dry-run         Preview without sending"
echo "    --json            Output as JSON"
echo ""
echo "  Supported Channels:"
echo "    • slack     - Slack workspace"
echo "    • discord   - Discord server"
echo "    • teams     - Microsoft Teams"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
