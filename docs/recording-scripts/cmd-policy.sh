#!/bin/bash
# relicta policy - Success and Error cases

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
echo -e "${CYAN}  relicta policy - Manage CGP governance policies${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# SUCCESS CASE 1: List policies
echo -e "${GREEN}━━━ Success: List active policies ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta policy list${NC}"
pause 0.5
echo ""
echo "  📋 Active Policies"
echo "  ────────────────────────────────────────"
echo ""
echo "  default"
echo "    Risk threshold:     0.7"
echo "    Auto-approve:       ≤ 0.3"
echo "    Require approval:   > 0.3"
echo "    Block:              > 0.7"
echo ""
echo "  production"
echo "    Risk threshold:     0.5"
echo "    Require approval:   Always"
echo "    Allowed actors:     ci-bot, release-manager"
echo ""
pause 3

# SUCCESS CASE 2: Show specific policy
echo ""
echo -e "${GREEN}━━━ Success: Show policy details ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta policy show default${NC}"
pause 0.5
echo ""
echo "  📜 Policy: default"
echo "  ────────────────────────────────────────"
echo ""
echo "  Risk Thresholds:"
echo "    auto_approve:       0.3"
echo "    require_approval:   0.7"
echo "    block:              0.9"
echo ""
echo "  Rules:"
echo "    • Breaking changes require approval"
echo "    • Major versions require approval"
echo "    • Security fixes auto-approved"
echo ""
echo "  Actors:"
echo "    Allowed: *"
echo "    Blocked: none"
echo ""
pause 3

# SUCCESS CASE 3: Validate policy
echo ""
echo -e "${GREEN}━━━ Success: Validate policy file ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta policy validate .relicta/policies/default.yaml${NC}"
pause 0.5
echo ""
echo "  ✓ Policy is valid"
echo "  ✓ All thresholds in range [0, 1]"
echo "  ✓ Actors list is valid"
echo "  ✓ No conflicting rules"
pause 3

# SUCCESS CASE 4: Evaluate current release against policy
relicta plan >/dev/null 2>&1
echo ""
echo -e "${GREEN}━━━ Success: Evaluate release against policy ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta policy eval${NC}"
pause 0.5
echo ""
echo "  🛡️  CGP Evaluation"
echo "  ────────────────────────────────────────"
echo ""
echo "  Policy:         default"
echo "  Risk Score:     0.42"
echo "  Decision:       require_approval"
echo ""
echo "  Factors:"
echo "    • Breaking changes:     +0.30"
echo "    • Multiple areas:       +0.12"
echo "    • Conventional commits: -0.10 (bonus)"
echo ""
echo "  Result: Manual approval required"
echo ""
pause 3

# ERROR CASE 1: Policy not found
echo ""
echo -e "${RED}━━━ Error: Policy not found ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta policy show nonexistent${NC}"
pause 0.5
echo ""
echo "  ✗ Policy 'nonexistent' not found"
echo ""
echo "  Available policies:"
echo "    • default"
echo "    • production"
pause 3

# ERROR CASE 2: Invalid policy file
echo ""
echo -e "${RED}━━━ Error: Invalid policy file ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta policy validate invalid.yaml${NC}"
pause 0.5
echo ""
echo "  ✗ Policy validation failed"
echo ""
echo "  Errors:"
echo "    • Line 5: 'auto_approve' must be less than 'require_approval'"
echo "    • Line 12: Invalid actor format"
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Subcommands:"
echo "    list              List all active policies"
echo "    show <name>       Show policy details"
echo "    validate <file>   Validate policy file"
echo "    eval              Evaluate current release"
echo ""
echo "  Policy Location:"
echo "    .relicta/policies/*.yaml"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3

# Cleanup
relicta cancel -f 2>/dev/null || true
