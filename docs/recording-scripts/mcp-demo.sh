#!/bin/bash
# MCP Demo: Agent-Managed Releases with Claude
#
# This script is an illustrative transcript, not a live MCP session. For an
# actual recording, use Claude Desktop with the MCP server running.

set -e
cd /tmp/relicta-demo

# Colors
CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

type_slow() {
    echo -n "$1" | while IFS= read -r -n1 char; do
        echo -n "$char"
        sleep 0.03
    done
    echo ""
}

pause() {
    sleep "${1:-1.5}"
}

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Relicta MCP: Agent-Managed Releases with Governance${NC}"
echo -e "${YELLOW}  Illustrative transcript — live values depend on your repository${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Scene 1: Setup
echo -e "${YELLOW}Step 1: Configure Claude Desktop${NC}"
echo ""
type_slow "Add Relicta to your Claude Desktop config:"
echo ""
pause 0.5

cat << 'EOF'
{
  "mcpServers": {
    "relicta": {
      "command": "relicta",
      "args": ["mcp", "serve"],
      "cwd": "/path/to/your/project"
    }
  }
}
EOF

pause 3

echo ""
echo -e "${GREEN}✓ Restart Claude Desktop to connect${NC}"
pause 2

# Scene 2: Status Check
clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Talking to Claude...${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${BLUE}You:${NC}"
type_slow "What's the release status for this project?"
pause 1

echo ""
echo -e "${GREEN}Claude:${NC} Let me check the current release status..."
echo ""
pause 0.5

echo -e "  ${YELLOW}→ Calling relicta_status${NC}"
pause 1

# Simulate status output
cat << 'EOF'

  📊 Release Status
  ─────────────────────────────────────
  Current Version:  v1.0.0
  State:            draft
  Commits Pending:  4

  Last Release:     v1.0.0 (2 days ago)
  Branch:           main

EOF

pause 2

echo -e "${GREEN}Claude:${NC} The project is at v1.0.0 with 4 commits pending release."
echo "        Would you like me to analyze them and plan the next version?"
pause 3

# Scene 3: Plan with Risk Assessment
echo ""
echo -e "${BLUE}You:${NC}"
type_slow "Yes, plan a release and show me the risk assessment"
pause 1

echo ""
echo -e "${GREEN}Claude:${NC} I'll analyze the commits and evaluate the risk..."
echo ""
pause 0.5

echo -e "  ${YELLOW}→ Calling relicta_plan --analyze${NC}"
pause 1.5

cat << 'EOF'

  📋 Release Plan
  ─────────────────────────────────────
  Next Version:     v2.0.0 (major)
  Release Type:     major

  Changes:
    ⚡ 1 breaking change
    ✨ 2 features
    🐛 1 fix
    📚 1 docs update

EOF

pause 2

echo -e "  ${YELLOW}→ Calling relicta_evaluate${NC}"
pause 1.5

cat << 'EOF'

  🛡️  CGP Risk Assessment
  ─────────────────────────────────────
  Decision:         require_approval

  Review reason:
    • A compatibility-breaking API change was detected

  Recommendation:
    ⚠️  Manual review required before publish

EOF

pause 3

echo -e "${GREEN}Claude:${NC} This is a major release due to the incompatible API v2 change."
echo "        The breaking change requires human review before publishing."
echo "        The breaking change is in the API module - shall I show the details?"
pause 3

# Scene 4: Generate Notes and Publish
echo ""
echo -e "${BLUE}You:${NC}"
type_slow "Generate release notes and I'll approve it for publishing"
pause 1

echo ""
echo -e "${GREEN}Claude:${NC} Drafting customer-facing notes from the plan and our product context..."
echo ""
pause 0.5

echo -e "  ${YELLOW}→ Calling relicta_notes with agent-authored draft${NC}"
pause 2

cat << 'EOF'

  📝 Release Notes (v2.0.0)
  ─────────────────────────────────────

  ## Compatibility

  - **API v2**: The new request API is not compatible with v1.
    Update existing integrations before adopting v2.

  ## Features

  - Added login and logout functions.
  - Added a farewell function alongside the greeting function.

EOF

pause 3

echo -e "${GREEN}Claude:${NC} Release notes generated. Ready to approve?"
pause 1

echo ""
echo -e "${BLUE}You:${NC}"
type_slow "Yes, approve and publish"
pause 1

echo ""
echo -e "${GREEN}Claude:${NC} Approving and publishing the release..."
echo ""

echo -e "  ${YELLOW}→ Calling relicta_approve${NC}"
pause 1
echo -e "  ${GREEN}✓ Release approved${NC}"
pause 0.5

echo -e "  ${YELLOW}→ Calling relicta_publish${NC}"
pause 2

cat << 'EOF'

  🚀 Release Published
  ─────────────────────────────────────
  Version:          v2.0.0
  Tag:              v2.0.0
  Changelog:        Updated

  Plugins Executed:
    ✓ github - Release created
    ✓ slack  - Team notified

EOF

pause 2

echo -e "${GREEN}Claude:${NC} Done! v2.0.0 has been published. The GitHub release is live"
echo "        and the team has been notified on Slack."
pause 3

# Closing
clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  ${GREEN}✨ AI Agents + Governance = Safe Releases${NC}"
echo ""
echo "  • Natural language release management"
echo "  • CGP risk assessment before every publish"
echo "  • Full audit trail of decisions"
echo "  • Works with Claude, GPT, and custom agents"
echo ""
echo -e "  ${BLUE}Learn more: relicta.dev/docs/mcp${NC}"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
