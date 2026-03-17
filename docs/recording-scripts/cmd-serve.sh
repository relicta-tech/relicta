#!/bin/bash
# relicta serve - Success and Error cases

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
echo -e "${CYAN}  relicta serve - Start the web dashboard${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
pause 2

# SUCCESS CASE 1: Start server on default port
echo -e "${GREEN}━━━ Success: Start dashboard on default port ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta serve${NC}"
pause 0.5
echo ""
echo "  🌐 Starting Relicta Dashboard..."
echo ""
echo "  Dashboard:  http://localhost:8080"
echo "  API:        http://localhost:8080/api/v1"
echo "  WebSocket:  ws://localhost:8080/api/v1/ws"
echo ""
echo "  Press Ctrl+C to stop"
pause 3

# SUCCESS CASE 2: Custom port
echo ""
echo -e "${GREEN}━━━ Success: Start on custom port ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta serve -p 3000${NC}"
pause 0.5
echo ""
echo "  🌐 Starting Relicta Dashboard..."
echo ""
echo "  Dashboard:  http://localhost:3000"
echo "  API:        http://localhost:3000/api/v1"
echo "  WebSocket:  ws://localhost:3000/api/v1/ws"
echo ""
echo "  Press Ctrl+C to stop"
pause 3

# SUCCESS CASE 3: With authentication
echo ""
echo -e "${GREEN}━━━ Success: Start with API key authentication ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta serve --api-key \${RELICTA_API_KEY}${NC}"
pause 0.5
echo ""
echo "  🌐 Starting Relicta Dashboard..."
echo ""
echo "  Dashboard:  http://localhost:8080"
echo "  Auth Mode:  API Key"
echo ""
echo "  Configure API keys in .relicta.yaml:"
echo "    dashboard:"
echo "      auth:"
echo "        mode: api_key"
echo "        api_keys:"
echo "          - key: \${RELICTA_API_KEY}"
echo "            name: admin"
echo ""
echo "  Press Ctrl+C to stop"
pause 3

# SUCCESS CASE 4: Bind to all interfaces
echo ""
echo -e "${GREEN}━━━ Success: Bind to all interfaces ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta serve --address 0.0.0.0:8080${NC}"
pause 0.5
echo ""
echo "  🌐 Starting Relicta Dashboard..."
echo ""
echo "  Dashboard:  http://0.0.0.0:8080"
echo "  Accessible from other machines on the network"
echo ""
echo "  ⚠️  Warning: Ensure firewall rules are configured"
echo ""
echo "  Press Ctrl+C to stop"
pause 3

# ERROR CASE 1: Port already in use
echo ""
echo -e "${RED}━━━ Error: Port already in use ━━━${NC}"
echo ""
echo -e "$ ${YELLOW}relicta serve -p 80${NC}"
pause 0.5
echo ""
echo "  ✗ Failed to start server"
echo "  Error: listen tcp :80: bind: address already in use"
echo ""
echo "  Try a different port: relicta serve -p 8080"
pause 3

# ERROR CASE 2: Not in a git repository
echo ""
echo -e "${RED}━━━ Error: Not in a git repository ━━━${NC}"
echo ""
echo "  cd /tmp && relicta serve"
echo ""
echo -e "$ ${YELLOW}relicta serve${NC}"
pause 0.5
echo ""
echo "  ✗ Not in a git repository"
echo "  Run 'relicta serve' from a git repository root"
pause 3

# Summary
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "  Flags:"
echo "    -p, --port        Port to listen on (default: 8080)"
echo "    -a, --address     Address to listen on (e.g., localhost:8080)"
echo "    -k, --api-key     Enable API-key auth with provided key"
echo "    -n, --no-auth     Disable authentication (not for production)"
echo ""
echo "  Dashboard Features:"
echo "    • Release pipeline visualization"
echo "    • Real-time status updates (WebSocket)"
echo "    • CGP risk analytics"
echo "    • Audit trail viewer"
echo "    • Approval workflow"
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
pause 3
