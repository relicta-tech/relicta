# Cognitive Backends (Mnemos + Chronos)

Relicta runs fully standalone by default. Mnemos and Chronos are optional
backends you can enable for enhanced release memory and pattern detection.

## What They Do

- **Mnemos** stores governance and release memory events for historical lookup.
- **Chronos** detects time-series patterns (trend, spike, drop, stall, anomaly).

## Prerequisites

- Relicta CLI installed
- Optional local services:
  - Mnemos running on `http://localhost:7777`
  - Chronos running on `http://localhost:7778`

## Run Locally

If you have both projects installed locally, start them in separate terminals.

```bash
# Terminal 1
mnemos serve

# Terminal 2
chronos serve
```

## Relicta Configuration

Add this to `.relicta.yaml`:

```yaml
governance:
  enabled: true
  memory_enabled: true

mnemos:
  enabled: true
  endpoint: http://localhost:7777
  timeout: 10s
  namespace: relicta

chronos:
  enabled: true
  endpoint: http://localhost:7778
  timeout: 10s
  metrics:
    - risk_score
    - release_frequency
    - lead_time
```

Notes:
- `mnemos.enabled: true` makes Mnemos the active governance memory backend.
- `chronos.enabled: true` enables Chronos client initialization for pattern analysis.

## Verify Setup

```bash
relicta plan --analyze
relicta evaluate
```

If a backend is unreachable, Relicta logs a warning and continues where possible.

## Troubleshooting

- Confirm endpoints are reachable:
  - `curl http://localhost:7777/health`
  - `curl http://localhost:7778/health`
- Check your `.relicta.yaml` for typos in `mnemos` and `chronos` sections.
- Re-run `relicta init` and compare generated config defaults.
