---
updated: 2026-06-13
note: append-only log — never edit or delete entries; supersede with "→ superseded [date]"
---
- 2026-06-13: Adopted Agent OS memory system — persistent cross-session state via memory/ + wiki/ + cadence skills.
- 2026-06-13: Attribution detection governs by *authorship*, not initiator. When a machine (agent/CI) authored a human-initiated release, the detected author governs the proposal so agent rules apply. Tightening-only; initiator preserved in audit context. Reputation guard still keys on initiator (separate concern). Gated behind governance.attribution_enabled, off by default.
- 2026-06-13: Reputation probation threshold and calibration accuracy floor are config-driven (governance.reputation_probation_threshold, calibration_min_accuracy/calibration_strict) rather than hardcoded. Zero values fall back to built-in defaults.
