#!/bin/bash
# Demo: Tutorial 4 - Configured AI Release Notes
# Requires a working OpenAI API key and demonstrates real provider output.

set -e

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
    echo "Set OPENAI_API_KEY to run this provider-backed AI notes demo." >&2
    exit 1
fi

CYAN='\033[0;36m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

# Use existing demo repo if available, otherwise create
DEMO_DIR="/tmp/relicta-demo"
if [ ! -d "$DEMO_DIR/.git" ]; then
    rm -rf "$DEMO_DIR"
    mkdir -p "$DEMO_DIR"
    cd "$DEMO_DIR"
    git init -q
    echo "# AI Notes Demo" > README.md
    git add . && git commit -q -m "initial"
    git tag -a v1.0.0 -m "v1.0.0"

    # Add commits with various types
    echo "export function login() {}" > auth.js
    git add . && git commit -q -m "feat(auth): add login function"

    echo "export function logout() {}" >> auth.js
    git add . && git commit -q -m "feat(auth): add logout function"

    echo "// BREAKING: redesigned API" > api.js
    git add . && git commit -q -m "feat(api)!: redesign API for v2"

    echo "export function goodbye() {}" > app.js
    git add . && git commit -q -m "fix(app): add farewell function"

    cat > .relicta.yaml << 'EOF'
versioning:
  strategy: conventional
ai:
  enabled: true
  provider: openai
EOF
    git add . && git commit -q -m "chore: config"
else
    cd "$DEMO_DIR"
fi

# Clean state
relicta cancel -f 2>/dev/null || true

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  AI-Powered Release Notes${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# Show commits
echo -e "${YELLOW}Commits since last release:${NC}"
git log --oneline v1.0.0..HEAD 2>/dev/null || git log --oneline -5
pause 2

# Step 1: Plan
echo ""
echo -e "${MAGENTA}━━━ Step 1: Analyze Changes ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta plan${NC}"
pause 1
relicta plan
pause 2

# Step 2: Bump
echo ""
echo -e "${MAGENTA}━━━ Step 2: Bump Version ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta bump${NC}"
pause 1
relicta bump
pause 2

# Step 3: AI Notes
echo ""
echo -e "${MAGENTA}━━━ Step 3: Generate AI-Powered Notes ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}  # ai.enabled: true in .relicta.yaml"
pause 1
NOTES_OUTPUT="$(relicta notes)"
printf '%s\n' "$NOTES_OUTPUT"
if ! grep -q 'Source: AI-generated (openai)' <<< "$NOTES_OUTPUT"; then
    echo "Relicta did not confirm OpenAI-generated notes; stopping to avoid showing a fallback as AI output." >&2
    exit 1
fi
pause 4

# Show changelog
echo ""
echo -e "${CYAN}Generated CHANGELOG.md:${NC}"
cat CHANGELOG.md 2>/dev/null | head -30 || echo "(Changelog would appear here)"
pause 4

# Cleanup
relicta cancel -f 2>/dev/null || true

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} AI analyzes commit context"
echo -e "  ${GREEN}✓${NC} Generates user-friendly summaries"
echo -e "  ${GREEN}✓${NC} Labels the configured provider in generated output"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
