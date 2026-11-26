#!/bin/bash
# Backward-compatible entry point for the maintained notes workflow demo.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/demo-ai-vs-template-notes.sh"
