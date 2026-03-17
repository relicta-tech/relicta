#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
rm -rf dist && mkdir -p dist

apps=(status pipeline risk commits approval blast)

for app in "${apps[@]}"; do
  echo "Building $app..."
  APP_ENTRY="$app" npx vite build --logLevel warn
done

# Flatten output structure
if [ -d dist/src/apps ]; then
  mv dist/src/apps/*.html dist/
  rm -rf dist/src
fi

# Copy to Go embed location
embed_dir="../internal/mcp/dist"
rm -rf "$embed_dir"
mkdir -p "$embed_dir"
cp dist/*.html "$embed_dir/"

echo "Built ${#apps[@]} apps to dist/ and copied to $embed_dir"
