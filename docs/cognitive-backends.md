# Cognitive Backends: Mnemos + Chronos

> **Enabled by default** — Relicta learns and improves with every release.
> Set `enabled: false` in config to opt-out.

Relicta integrates two cognitive backends that work together to provide
memory and pattern detection for smarter release governance.

## Quick Start

Both backends are **enabled by default** in v4.0.0+. No configuration needed.

To opt-out, add to your `.relicta.yaml`:

```yaml
mnemos:
  enabled: false  # Disable Mnemos memory backend

chronos:
  enabled: false  # Disable Chronos pattern detection
```

## Mnemos — Release Memory

Mnemos (https://github.com/felixgeelhaar/mnemos) is a self-hosted memory layer
for AI applications. When enabled, Relicta stores:

- Release events (version, risk score, decision, outcome)
- Incident records (type, severity, root cause)
- Governance decisions (decision, approvals, authorized steps)
- Execution authorizations (who approved, what's allowed)

### Setup (optional — for enhanced features)

If you want to run Mnemos as a separate service for advanced querying:

```bash
# Install Mnemos
go install github.com/felixgeelhaar/mnemos/cmd/mnemos@latest

# Start Mnemos server (default: localhost:7777)
mnemos serve
```

Relicta will connect to `http://localhost:7777` by default.
If Mnemos is not running, operations gracefully degrade (warn-level logging).

## Chronos — Pattern Detection

Chronos (https://github.com/felixgeelhaar/chronos) detects patterns in
time-series data: recurrence, trend, spike, drop, stall, anomaly.

When enabled, Relicta sends release metrics (risk score, deployment frequency,
outcome) to Chronos and queries for patterns to improve risk scoring.

### Setup (optional — for pattern detection)

If you want to run Chronos as a separate service for advanced analytics:

```bash
# Install Chronos
go install github.com/felixgeelhaar/chronos/cmd/chronos@latest

# Start Chronos server (default: localhost:7778)
chronos serve
```

Relicta will connect to `http://localhost:7778` by default.
If Chronos is not running, operations gracefully degrade (warn-level logging).

## Configuration

Both backends are configured in `.relicta.yaml`:

```yaml
# Mnemos memory backend (enabled by default)
mnemos:
  enabled: true          # Set false to opt-out
  endpoint: "http://localhost:7777"
  timeout: 10s
  namespace: "relicta"  # Used as Mnemos run_id

# Chronos pattern detection (enabled by default)
chronos:
  enabled: true          # Set false to opt-out
  endpoint: "http://localhost:7778"
  timeout: 10s
  metrics:               # Metrics to analyze for patterns
    - "risk_score"
    - "release_frequency"
    - "lead_time"
```

## Graceful Degradation

Both adapters fail gracefully:
- If the backend is not running, operations become no-ops
- Warn-level logs are emitted (not errors)
- Relicta continues to function normally
- Features that require the backend will return empty results

This means you can use Relicta without running Mnemos/Chronos,
and enable them later when you want enhanced memory and pattern detection.

## Default Behavior (v4.0.0+)

✅ **Mnemos enabled by default** — learns from every release
✅ **Chronos enabled by default** — detects patterns and trends
✅ **Graceful degradation** — no separate `serve` commands needed
✅ **Explicit opt-out** — set `enabled: false` to disable

## Migration from v3.x

If upgrading from v3.x, update your config:

```yaml
# Add to your .relicta.yaml (or rely on defaults)
mnemos:
  enabled: true  # Now enabled by default

chronos:
  enabled: true  # Now enabled by default
```