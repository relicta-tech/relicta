#!/bin/bash
# Demo: Conventional Commits vs Messy Commits
#
# This demo shows how Relicta handles:
# 1. Well-formatted conventional commits
# 2. Real-world messy commit history
#
# Proves that Relicta works great even without perfect commits!

set -e

CYAN='\033[0;36m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m'

pause() { sleep "${1:-1.5}"; }

clear
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Conventional vs Messy Commits${NC}"
echo -e "${CYAN}  Relicta handles both!${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

#############################################
# PART 1: Conventional Commits (Clean Repo)
#############################################

echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  PART 1: Repository with Conventional Commits${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 1

# Create clean conventional commits repo
rm -rf /tmp/conventional-demo
mkdir -p /tmp/conventional-demo
cd /tmp/conventional-demo
git init -q

# Create initial commit and tag
echo "# My App" > README.md
git add . && git commit -q -m "feat: initial project setup"
git tag v1.0.0

# Add conventional commits
echo "function auth() {}" > auth.js
git add . && git commit -q -m "feat(auth): add user authentication

- Add login/logout functionality
- Support for JWT tokens
- Session management"

echo "function login() { /* fixed */ }" > login.js
git add . && git commit -q -m "fix(auth): resolve token expiration bug

Tokens were not being refreshed properly after
the session timeout. This caused users to be
logged out unexpectedly.

Fixes #123"

echo "# API Docs" > API.md
git add . && git commit -q -m "docs: add API documentation for auth endpoints"

echo "// Breaking: new API" > api.js
git add . && git commit -q -m "feat!: redesign authentication API

BREAKING CHANGE: The auth API now uses OAuth 2.0
instead of the legacy token system. See migration
guide in docs/migration.md"

# Initialize relicta
cat > .relicta.yaml << 'EOF'
versioning:
  strategy: conventional
EOF

echo -e "${YELLOW}Commit history:${NC}"
echo ""
git log --oneline v1.0.0..HEAD
echo ""
pause 3

echo -e "$ ${YELLOW}relicta plan -a${NC}"
pause 0.5
relicta plan -a 2>/dev/null || echo "  (plan output would appear here)"
pause 3

echo ""
echo -e "${GREEN}✓ Perfect parsing: types, scopes, breaking changes detected!${NC}"
pause 3

#############################################
# PART 2: Messy Commits (Real World Repo)
#############################################

echo ""
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${MAGENTA}  PART 2: Repository with Messy Commits${NC}"
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 1

# Create messy commits repo
rm -rf /tmp/messy-demo
mkdir -p /tmp/messy-demo
cd /tmp/messy-demo
git init -q

# Create initial commit and tag
echo "# My App" > README.md
git add . && git commit -q -m "initial commit"
git tag v1.0.0

# Add messy commits (realistic!)
echo "function login() {}" > login.js
git add . && git commit -q -m "wip"

echo "function logout() {}" >> login.js
git add . && git commit -q -m "add logout"

echo "// fix" >> login.js
git add . && git commit -q -m "fixed the thing"

echo "// more fixes" >> login.js
git add . && git commit -q -m "actually fixed it this time lol"

echo "function auth() {}" > auth.js
git add . && git commit -q -m "Update auth.js"

echo "// tests" > test.js
git add . && git commit -q -m "stuff"

echo "// breaking" > api.js
git add . && git commit -q -m "BREAKING: new api!!!"

echo "// cleanup" >> api.js
git add . && git commit -q -m "cleanup and refactor some things"

# Initialize relicta
cat > .relicta.yaml << 'EOF'
versioning:
  strategy: conventional
EOF

echo -e "${YELLOW}Commit history (messy):${NC}"
echo ""
git log --oneline v1.0.0..HEAD
echo ""
echo "  😱 No conventional commits!"
echo "  😱 Vague messages like 'wip', 'stuff', 'fixed the thing'"
echo "  😱 Inconsistent formatting"
echo ""
pause 3

echo -e "$ ${YELLOW}relicta plan -a${NC}"
pause 0.5
relicta plan -a 2>/dev/null || echo "  (plan output would appear here)"
pause 3

echo ""
echo -e "${GREEN}✓ Relicta still works! Uses heuristics to:${NC}"
echo "    • Detect 'BREAKING' keyword"
echo "    • Identify fix-like commits"
echo "    • Group related changes"
echo "    • Fall back to 'patch' for unclear commits"
pause 4

# Now show release notes on the messy repo
echo ""
echo -e "${BLUE}━━━ Release notes from messy history ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta notes${NC}"
pause 0.5
relicta plan >/dev/null 2>&1
relicta bump >/dev/null 2>&1
relicta notes
pause 4

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Relicta handles ANY commit history:"
echo ""
echo -e "  ${GREEN}Conventional Commits:${NC}"
echo "    • Perfect type detection (feat, fix, docs...)"
echo "    • Scope parsing (auth, api, core...)"
echo "    • Breaking change detection from footer"
echo "    • Issue linking (#123)"
echo ""
echo -e "  ${MAGENTA}Messy Commits:${NC}"
echo "    • Keyword detection (BREAKING, fix, add...)"
echo "    • Intelligent grouping"
echo "    • Sensible fallback to patch"
echo "    • Optional configured AI can add prose; review its claims"
echo ""
echo -e "  ${BLUE}Best of both worlds:${NC}"
echo "    • Works with any team's workflow"
echo "    • No need to rewrite history"
echo "    • Gradually adopt conventions"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 4

# Cleanup
rm -rf /tmp/conventional-demo /tmp/messy-demo
