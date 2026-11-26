#!/bin/bash
# Demo: Deterministic notes and customer-facing MCP drafts
#
# The CLI generates deterministic notes by default. For customer-facing copy,
# a connected coding agent can draft from the product context in its session
# and persist the exact draft through Relicta MCP.

set -e
cd /tmp/relicta-demo

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Deterministic Notes and MCP Drafts${NC}"
echo -e "${CYAN}  Evidence from Relicta, product context from your agent session${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Ensure clean state
relicta cancel -f 2>/dev/null || true

# Setup
echo -e "${YELLOW}Setting up release...${NC}"
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
echo -e "${GREEN}✓ Ready to generate notes${NC}"
echo ""
pause 2

# Show the commits we're working with
echo -e "${BLUE}━━━ Commits being analyzed ━━━${NC}"
echo ""
git log --oneline v1.0.0..HEAD 2>/dev/null || git log --oneline -5
echo ""
pause 3

# Template-based notes (default)
echo ""
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${MAGENTA}  TEMPLATE-BASED NOTES (default)${NC}"
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 1
relicta notes
pause 4

# Explain the agent-session path without presenting fabricated model output.
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  CUSTOMER-FACING NOTES IN YOUR MCP AGENT SESSION${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "This demo repository has AI note generation disabled. For customer copy:"
echo "  1. Call relicta_plan and use its recommendation as the factual source."
echo "  2. Call relicta_bump to set the version before saving notes."
echo "  3. Draft from product context already established in the agent session."
echo "  4. Call relicta_notes with the draft before approval."
echo "  5. Read back the saved notes with relicta_status for review."
echo ""
echo "The agent-authored draft is stored as supplied and does not claim"
echo "that Relicta fact-checks customer-facing prose."
pause 4

# Comparison summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Which path to use:"
echo ""
echo -e "  ${MAGENTA}Deterministic CLI Notes:${NC}"
echo "    • Repeatable output from the classified changes"
echo "    • Useful for changelog and technical release records"
echo "    • No separate AI provider required"
echo ""
echo -e "  ${GREEN}MCP Agent-Authored Notes:${NC}"
echo "    • Uses product context from the existing agent conversation"
echo "    • Grounds claims in the Relicta recommendation"
echo "    • Saves the exact draft for review and approval"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  See ${YELLOW}docs/mcp.md${NC} for the full agent-session workflow"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 4

# Cleanup
relicta cancel -f 2>/dev/null || true
