# Relicta Dashboard

The Relicta Dashboard is a self-hosted web UI that provides real-time visibility into your release governance workflow. It's embedded directly in the CLI binary for single-file deployment.

## Quick Start

```bash
# Build with embedded frontend
make build-with-frontend

# Start the dashboard
./bin/relicta serve

# Or specify a port
./bin/relicta serve --port 3000
```

The dashboard will be available at `http://localhost:8080` (or your specified port).

## Features

### Release Pipeline
- View all releases with state, risk score, and progress
- Filter by state (draft, planned, versioned, etc.) and risk level
- Real-time updates via WebSocket
- Detailed view with commits, events, and approval info

### Governance Analytics
- Risk score trends over time
- Risk factor distribution
- Decision history (approve/deny/require review)
- Configurable time ranges

### Team Performance
- Actor metrics and reliability scores
- Success rates and average risk scores
- Trust level tracking (trusted, standard, probation)
- Sortable columns

### Approval Workflow
- Pending approvals queue
- One-click approve/reject with justification
- Review reasons for flagged releases
- Recent decision history

### Audit Trail
- Searchable event log
- Filter by type, actor, release, or date range
- CSV export capability
- Detailed event data inspection

## Configuration

Configure the dashboard in your `release.config.yaml`:

```yaml
dashboard:
  enabled: true
  address: ":8080"

  auth:
    mode: api_key  # none | api_key
    api_keys:
      - key: ${RELICTA_DASHBOARD_KEY}
        name: "Admin"
        roles: ["admin"]
      - key: ${RELICTA_VIEWER_KEY}
        name: "Viewer"
        roles: ["viewer"]

  timeouts:
    read: 15s
    write: 15s
    idle: 60s
```

### Authentication Modes

- **none**: No authentication (not recommended for production)
- **api_key**: Require API key in Authorization header or query param

### Environment Variables

| Variable | Description |
|----------|-------------|
| `RELICTA_DASHBOARD_ADDRESS` | Server address (e.g., `:8080`) |
| `RELICTA_DASHBOARD_KEY` | API key for dashboard access |

## API Endpoints

### Releases
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/releases` | List all releases |
| GET | `/api/v1/releases/:id` | Get release details |
| GET | `/api/v1/releases/:id/events` | Get release events |
| GET | `/api/v1/releases/active` | Get active release |

### Governance
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/governance/decisions` | List decisions |
| GET | `/api/v1/governance/risk-trends` | Get risk trends |
| GET | `/api/v1/governance/factors` | Get factor distribution |

### Approvals
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/approvals/pending` | List pending approvals |
| POST | `/api/v1/approvals/:id/approve` | Approve a release |
| POST | `/api/v1/approvals/:id/reject` | Reject a release |

### Actors
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/actors` | List all actors |
| GET | `/api/v1/actors/:id` | Get actor details |

### Audit
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/audit` | Query audit events |

### WebSocket
| Endpoint | Description |
|----------|-------------|
| `/api/v1/ws` | Real-time event stream |

## WebSocket Events

Connect to `/api/v1/ws` to receive real-time updates:

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ws?api_key=YOUR_KEY');

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log(message.type, message.payload);
};
```

### Event Types

| Event | Description |
|-------|-------------|
| `release.created` | New release started |
| `release.state_changed` | Release state updated |
| `release.versioned` | Version assigned |
| `release.approved` | Release approved |
| `release.published` | Release published |
| `release.failed` | Release failed |
| `release.canceled` | Release canceled |
| `release.step_completed` | Step completed |
| `release.plugin_executed` | Plugin executed |

## Building

### API-Only Mode

Build without frontend for API-only deployment:

```bash
make build
```

### With Embedded Frontend

Build with Vue frontend embedded:

```bash
make build-with-frontend
```

### Development

For frontend development with hot reload:

```bash
# Terminal 1: Start API server
./bin/relicta serve --port 8080

# Terminal 2: Start Vite dev server
cd web
npm run dev
```

The Vite dev server proxies API requests to the backend.

## Architecture

```
┌─────────────────────────────────────┐
│         Vue 3 + TypeScript          │
│      (embedded via go:embed)        │
└─────────────────┬───────────────────┘
                  │
┌─────────────────┴───────────────────┐
│          Chi HTTP Router            │
├─────────────┬───────────┬───────────┤
│  REST API   │ WebSocket │  Static   │
│  Handlers   │    Hub    │  Files    │
└──────┬──────┴─────┬─────┴───────────┘
       │            │
┌──────┴────────────┴─────────────────┐
│           Domain Services           │
│       Release · Governance          │
└─────────────────────────────────────┘
```

### Key Components

- **HTTP Server**: Chi router with middleware for auth, CORS, logging
- **WebSocket Hub**: Manages client connections and broadcasts events
- **Event Broadcaster**: Bridges domain events to WebSocket clients
- **Vue Frontend**: Single-page app with Pinia stores and Vue Router

## Theming

The dashboard supports light and dark themes:

- Toggle via the sun/moon icon in the header
- Preferences are persisted in localStorage
- System theme detection for "Auto" mode
- Full Tailwind CSS dark mode support

## Security

- API key authentication for all endpoints
- Bearer token or query parameter authentication
- CORS configuration for cross-origin requests
- WebSocket authentication via query parameter

## Troubleshooting

### Dashboard shows "API-only mode"

The binary was built without the frontend. Rebuild with:

```bash
make build-with-frontend
```

### WebSocket connection fails

1. Check API key is correct
2. Verify the server is running
3. Check browser console for CORS errors

### No data showing

1. Ensure release services are initialized
2. Check that you're in a git repository
3. Verify the release.config.yaml exists

## Future Evolution

The embedded dashboard architecture is intentional for the current phase:

| Use Case | Embedded Works Well |
|----------|---------------------|
| Local development | ✓ |
| Single-user / small team | ✓ |
| Quick demos | ✓ |
| Self-hosted single instance | ✓ |

For production enterprise scenarios (multi-user, SSO, horizontal scaling), the architecture will evolve to a standalone service deployment. See `docs/internal/prd.md` §14.1 for the full migration roadmap.

The REST API is designed to remain stable across this evolution.
