#!/bin/bash
# Setup a demo repository for recording videos

set -e

DEMO_DIR="/tmp/relicta-demo"

echo "Setting up demo repository at $DEMO_DIR..."

# Clean up existing demo
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"
cd "$DEMO_DIR"

# Initialize git repo
git init
git config user.email "demo@relicta.dev"
git config user.name "Demo User"

# Create initial structure
mkdir -p src tests docs

cat > package.json << 'EOF'
{
  "name": "demo-app",
  "version": "1.0.0",
  "description": "Demo application for Relicta videos"
}
EOF

cat > src/app.js << 'EOF'
// Demo application
export function greet(name) {
  return `Hello, ${name}!`;
}
EOF

cat > README.md << 'EOF'
# Demo App

A demo application for Relicta video recordings.
EOF

# Initial commit
git add .
git commit -m "chore: initial commit"

# Create v1.0.0 tag with message (required if gpgsign is enabled globally)
git tag -a v1.0.0 -m "Release v1.0.0" --no-sign

# Add feature commits
cat > src/auth.js << 'EOF'
// Authentication module
export function login(user, pass) {
  return { token: 'abc123', user };
}

export function logout() {
  return true;
}
EOF
git add .
git commit -m "feat(auth): add login and logout functions"

# Add fix commit
cat >> src/app.js << 'EOF'

export function farewell(name) {
  return `Goodbye, ${name}!`;
}
EOF
git add .
git commit -m "fix(app): add missing farewell function"

# Add docs commit
cat >> README.md << 'EOF'

## Features

- User authentication
- Greeting and farewell functions
EOF
git add .
git commit -m "docs: update README with features"

# Add breaking change commit
cat > src/api.js << 'EOF'
// BREAKING CHANGE: New API structure
export const API_VERSION = 2;

export function request(endpoint, options) {
  return fetch(endpoint, options);
}
EOF
git add .
git commit -m "feat(api)!: redesign API with breaking changes

BREAKING CHANGE: API v2 is not compatible with v1"

# Create .gitignore for relicta state
cat > .gitignore << 'EOF'
.relicta/
EOF
git add .gitignore
git commit -m "chore: add .gitignore"

# Create relicta config
cat > .relicta.yaml << 'EOF'
versioning:
  strategy: conventional
  tag_prefix: v
  git_tag: true
  git_push: false  # Don't push in demos

changelog:
  file: CHANGELOG.md
  format: keep-a-changelog

ai:
  enabled: false

governance:
  enabled: true
EOF

git add .relicta.yaml
git commit -m "chore: add relicta configuration"

echo ""
echo "Demo repository created at $DEMO_DIR"
echo "Commits since v1.0.0:"
git log --oneline v1.0.0..HEAD
echo ""
echo "Ready for recording!"
