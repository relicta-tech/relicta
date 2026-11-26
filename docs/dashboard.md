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

The dashboard binds to loopback by default and will be available at `http://localhost:8080` (or your specified port). In `none` mode, anyone who can reach that address can view dashboard data; access is read-only. Keep this mode on loopback. Use `--address` or `dashboard.address` to bind externally only after configuring authentication.

### Standalone Dashboard Container

The optional Nginx dashboard image can be built from the repository and placed
on the same Docker network as a Relicta server. The API server must use an
authenticated mode such as `api_key`; users enter that key in the dashboard's
sign-in screen. Set `RELICTA_API_URL` to a hostname the Nginx container can
resolve on that network. Its default is `http://relicta:8080`, so the server
must be reachable as `relicta` when using the default.

```bash
# Run these commands from the repository root.
docker build -f web/docker/Dockerfile -t relicta-dashboard:local web

# Attach to the existing Docker network that contains the Relicta API server.
docker run --rm --name relicta-dashboard \
  --network relicta-net \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=16m,mode=1777 \
  --tmpfs /run:rw,noexec,nosuid,size=8m,uid=101,gid=101,mode=755 \
  --tmpfs /var/cache/nginx:rw,noexec,nosuid,size=32m,uid=101,gid=101,mode=755 \
  --tmpfs /etc/nginx/conf.d:rw,noexec,nosuid,size=1m,uid=101,gid=101,mode=755 \
  --cap-drop=ALL --security-opt=no-new-privileges \
  --pids-limit=100 --memory=256m --cpus=1 \
  -e RELICTA_API_URL=http://relicta:8080 \
  -p 127.0.0.1:8081:80 \
  relicta-dashboard:local
```

The writable tmpfs mounts are required because Nginx expands its proxy
configuration at startup and writes its PID and cache files. Keep the host
port on loopback when using a local TLS proxy; for a shared deployment, expose
the dashboard only through a TLS-terminating proxy and restrict direct network
access. The container health endpoint is `/nginx-health`.

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

Configure the dashboard in your `.relicta.yaml`:

```yaml
dashboard:
  enabled: true
  # Local access by default. For a remote deployment, bind to a reachable
  # interface and keep authentication enabled.
  address: "127.0.0.1:8080"

  auth:
    mode: none  # none | api_key | session | oidc
    # For remote access, use api_key, session, or oidc. Example API key setup:
    # mode: api_key
    # api_keys:
    #   - key: ${RELICTA_DASHBOARD_KEY}
    #     name: "Admin"
    #     roles: ["admin"]

  timeouts:
    read: 15s
    write: 15s
    idle: 60s
```

### Authentication Modes

- **none**: No authentication; grants anonymous read-only access to dashboard data (local development only)
- **api_key**: Require API key in `X-API-Key` or `Authorization: Bearer` header; the server refuses to start if no non-empty API key is configured
- **session**: Username/password API login that issues signed JWT access and refresh tokens; configure users and a strong `session_secret`.
- **oidc**: OpenID Connect authorization-code login endpoints that issue a local JWT session; configure `session_secret` and the OIDC provider settings.

The embedded dashboard UI provides API-key, password-session, and OIDC sign-in. Session access and refresh tokens are kept in tab-scoped browser storage and refreshed through the server's token-rotation endpoint. Sign-out clears tab storage immediately and asks the server to revoke both tokens. OIDC browser sign-in uses an HttpOnly state cookie and a short-lived, single-use ticket; API clients can continue to use the JSON callback. Configure the provider issuer, client credentials, HTTPS redirect URL, and role mappings under `dashboard.auth.oidc`.

Relicta does not currently provide a fully highly available dashboard
deployment. With PostgreSQL and the same session secret, authenticated API
requests and session revocation can be shared across replicas. OIDC login,
callback, and browser ticket exchange depend on short-lived process-local state,
so configure load-balancer session affinity for those requests. WebSocket and
server-sent event broadcasts are also local to each instance; clients connected
to another replica will not receive those broadcasts. A shared database alone
does not make the dashboard stateless. For the complete dashboard experience,
use one instance. Put remote access behind a TLS-terminating reverse proxy,
restrict network access to that proxy, and configure the public origin in
`dashboard.auth.oidc.redirect_url`.

For a dashboard exposed through an HTTPS reverse proxy, configure the externally
reachable callback URL explicitly and register that exact URL with the identity
provider:

```yaml
dashboard:
  address: "127.0.0.1:8080"
  auth:
    mode: oidc
    session_secret: "${RELICTA_DASHBOARD_SESSION_SECRET}"
    oidc:
      issuer_url: "https://id.example.com"
      client_id: "relicta-dashboard"
      client_secret: "${RELICTA_OIDC_CLIENT_SECRET}"
      redirect_url: "https://relicta.example.com/api/v1/auth/oidc/callback"
      groups_claim: groups
      default_role: viewer
      claim_mappings:
        - claim: groups
          value: relicta-admins
          role: admin
        - claim: groups
          value: relicta-approvers
          role: approver
```

Use a randomly generated session secret of at least 32 bytes, keep it private,
and set the same value on every replica. Keep TLS enabled at the public proxy
boundary. Session and OIDC modes refuse to start when the secret is shorter
than 32 bytes.
OIDC issuer and callback URLs must use HTTPS; plain HTTP is accepted only for
loopback development providers and callbacks. Issuer URLs cannot contain
credentials, query parameters, or fragments. OIDC flow state and short-lived
browser exchange tickets remain in process memory, so keep session affinity for
OIDC login, callback, and ticket exchange. Token revocation and refresh
rotation use the configured persistence backend: `file` and `sqlite` store
records in `.relicta/relicta.db`, while `postgres` stores them in the shared
`token_revocations` table. SQLite survives local server restarts; use PostgreSQL
for replicas that need consistent revocation across hosts. PostgreSQL with
`migration_mode: manual` requires `relicta db migrate` before starting a
session-authenticated dashboard. The server fails to start if durable session
storage cannot be opened, and authentication fails closed if that store becomes
unavailable. Keep the session secret unchanged across restarts to avoid
invalidating every session.

Default behavior:
- The default config uses `none` on `127.0.0.1:8080` for local development.
- An unauthenticated non-loopback bind is refused unless `allow_insecure_no_auth: true` is explicitly set. That override exposes dashboard data without authentication and is not suitable for shared or production environments. Configure `api_key`, `session`, or `oidc` for remote access.
- `GET /api/v1/health/cognitive` reports whether Mnemos and Chronos are enabled in the loaded configuration. It requires dashboard authentication and does not mean either external service is reachable.

### Secret Environment Variables

Dashboard credential values support the same `${VAR}` expansion as other
secret-bearing configuration fields. Set the variable in the process environment
and reference it in `.relicta.yaml`; the variable names below are examples and
are not loaded automatically as standalone Relicta settings.

| Variable | Example configuration | Description |
|----------|-----------------------|-------------|
| `RELICTA_DASHBOARD_KEY` | `dashboard.auth.api_keys[].key: "${RELICTA_DASHBOARD_KEY}"` | API key for dashboard access |
| `RELICTA_DASHBOARD_SESSION_SECRET` | `dashboard.auth.session_secret: "${RELICTA_DASHBOARD_SESSION_SECRET}"` | Random secret of at least 32 bytes, used to sign session tokens |
| `RELICTA_OIDC_CLIENT_SECRET` | `dashboard.auth.oidc.client_secret: "${RELICTA_OIDC_CLIENT_SECRET}"` | OIDC client secret |

Set the server address in `.relicta.yaml` or with `relicta serve --address`.

For example, to run a containerized server with API key authentication, mount a
configuration file containing `address: "0.0.0.0:8080"` and `auth.mode:
api_key`, provide the configured key through your secret manager, and publish
the container port only on the intended host interface. The server refuses a
non-loopback bind with `auth.mode: none` unless `allow_insecure_no_auth` is
explicitly enabled.

## API Endpoints

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/auth/mode` | Read the configured auth mode for the embedded sign-in screen |
| POST | `/api/v1/auth/login` | Exchange configured username/password credentials for a session |
| POST | `/api/v1/auth/refresh` | Rotate session access and refresh tokens |
| POST | `/api/v1/auth/logout` | Revoke the access token and, when supplied, its paired refresh token |
| GET | `/api/v1/auth/oidc/login` | Start OIDC login; add `response_mode=fragment` for the embedded browser flow |
| GET | `/api/v1/auth/oidc/callback` | Complete OIDC login (JSON for API clients, one-use ticket redirect for browser flow) |
| POST | `/api/v1/auth/oidc/exchange` | Exchange the browser flow's one-use ticket for a session |

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
| `POST /api/v1/ws/ticket` | Exchange dashboard authentication for a single-use WebSocket ticket |
| `/api/v1/ws` | Real-time event stream; requires a valid ticket |

## WebSocket Events

Request a short-lived ticket, then connect to `/api/v1/ws` to receive real-time updates:

```javascript
const response = await fetch('/api/v1/ws/ticket', {
  method: 'POST',
  headers: { 'X-API-Key': 'YOUR_API_KEY' },
  cache: 'no-store',
});
const { ticket } = await response.json();
const ws = new WebSocket(`ws://localhost:8080/api/v1/ws?ticket=${encodeURIComponent(ticket)}`);

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log(message.type, message.payload);
};
```

Tickets expire after 30 seconds, are single-use, and are returned with cache
prevention headers. The browser URL contains only the ticket, not the API key or
session token. Use `wss://` for remote connections. API keys must be sent in
`X-API-Key` or `Authorization: Bearer`; query-string API-key authentication is
not supported.

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

### Standalone Frontend Container

The optional frontend image serves the dashboard and proxies `/api/` requests,
including WebSocket connections, to a running Relicta server. Configure the
backend URL when starting the container; the backend must use an authenticated
dashboard mode and be reachable from the frontend container:

```bash
docker build -f web/docker/Dockerfile -t relicta-dashboard:local web
docker run --rm -p 8081:80 \
  -e RELICTA_API_URL=http://host.docker.internal:8080 \
  --add-host=host.docker.internal:host-gateway \
  relicta-dashboard:local
```

Open `http://localhost:8081`. On Docker Desktop, `host.docker.internal` is
provided by the platform; `--add-host` supplies it on Linux. Set
`RELICTA_API_URL` to the backend's origin (scheme, host, and port, without an
API path). Keep the backend protected with API-key, session, or OIDC
authentication and expose both services only through the intended network and
TLS boundary. The frontend image does not include or replace the Relicta API
server.

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
- Bearer token or `X-API-Key` header authentication
- CORS configuration for cross-origin requests
- WebSocket upgrades require short-lived, single-use tickets issued after the
  configured dashboard authentication succeeds.

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
3. Verify the `.relicta.yaml` exists

## Deployment support boundary

The dashboard supports local development, demos, and a self-hosted single
instance. API-key, password-session, and OIDC authentication are available, but
that does not imply enterprise high availability. PostgreSQL-backed API
sessions can be shared across instances, but OIDC browser login requires
session affinity and live WebSocket/SSE broadcasts remain instance-local. The
dashboard does not provide a supported high-availability deployment or shared
OIDC ticket storage. Use the CLI or MCP server where the single-instance
dashboard deployment model does not meet your availability requirements.
