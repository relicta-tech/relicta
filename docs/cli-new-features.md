# New CLI Features & Cognitive Backend Enhancements

This document covers the new CLI flags, endpoints, and commands added in the v4.0.0+ enhancement cycle.

## New CLI Flags

### Global Flags
| Flag | Alias | Description | Default |
|------|--------|-------------|---------|
| `--log-level` | `--log` | Log level (debug, info, warn, error) | `info` |
| `--skip-cognitive` | | Skip Mnemos & Chronos backends for this command | `false` |
| `--chronos-threads` | | Max concurrent Chronos ingest requests (overrides config) | `4` (config) |

### `relicta plan` Specific Flags
| Flag | Description | Notes |
|------|-------------|-------|
| `--skip-cognitive` | Disables Mnemos/Chronos adapters for this plan run | Sets `config.Mnemos.Enabled = false` and `config.Chronos.Enabled = false` |
| `--chronos-threads N` | Overrides the `Chronos.Threads` config value | Controls semaphore size for ingest concurrency |

### `relicta version` Specific Flags
| Flag | Description | Output |
|------|-------------|--------|
| `--cognitive` | Probes Mnemos & Chronos health endpoints | Prints `mnemos: healthy/unreachable (url)` and `chronos: healthy/unreachable (url)` |

## New Commands

### `relicta demo`
Manage a local Docker Compose environment for product showcases.

**Flags:**
- `-f, --file` : Path to Docker Compose file (default: `docker-compose.demo.yml`)
- `--reset` : Run `docker compose down -v` before starting
- `--down` : Tear down the demo environment

**Examples:**
```bash
relicta demo
relicta demo --reset
relicta demo --down
```

**Prerequisites:** Docker and Docker Compose must be installed.
**Default Compose File:** `docker-compose.demo.yml` (auto-created in repo root)

## New HTTP Endpoints

### `GET /health/cognitive`
Reports the status of the Mnemos and Chronos cognitive backends.

**Response:**
```json
{
  "mnemos": "enabled",
  "chronos": "enabled"
}
```
*Note: Currently returns static "enabled" status. Future versions will ping actual backend health.*

## Configuration Changes

### ChronosConfig Additions
```yaml
chronos:
  enabled: true
  endpoint: "http://localhost:7778"
  timeout: 10s
  threads: 4          # New: concurrency for ingest
  metrics:
    - risk_score
    - release_frequency
    - lead_time
```

## Behavior Changes
1. **Graceful Degradation:** Both Mnemos and Chronos adapters now fail-soft. If the backend is unavailable, they log a warning and continue instead of returning errors.
2. **Mnemos Record ID Prefix:** Release records are stored with `source_input_id` prefixed as `release-<id>`, incidents as `incident-<id>`, etc.
3. **CI Workflow:** Nox security scan is now enforced (removed `|| true`). The `lint` and `test` jobs are merged into a single `lint-and-test` job with shared Go module caching.

## Makefile Additions
- `make changelog` : Generates `docs/changelog.md` from git commits since the last tag.
