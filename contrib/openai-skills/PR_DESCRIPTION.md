## Summary
Add new skill `relicta-release-governance` to `skills/.experimental`.

This skill helps Codex run Relicta release workflows safely and consistently, including planning, version bumping, notes generation, approval, publishing, plugin operations, MCP usage, dashboard operations, and release-state recovery.

## Skill Path
- `skills/.experimental/relicta-release-governance/`

## Why
Relicta has a multi-step release/governance workflow where command order and state handling are important. This skill codifies a deterministic playbook to reduce operator error and speed up troubleshooting.

## What Is Included
- `SKILL.md` with:
  - End-to-end release workflow
  - Safety checks and dry-run guidance
  - Governance/policy checkpoints
  - Plugin and MCP/dashboard usage
  - Recovery playbook (`status`, `history`, `cancel`, `reset`, `clean`)
- `agents/openai.yaml` with UI metadata

## Validation
- Ran skill validator:
  - `quick_validate.py skills/.experimental/relicta-release-governance`
  - Result: `Skill is valid!`

## Notes
- Config filename references are standardized to `.relicta.yaml`.
- Dashboard auth guidance explicitly treats `none` as local-development only and recommends `api_key` for shared environments.

## Suggested Follow-Up
If adoption is strong and feedback is positive, promote this skill from `.experimental` to `.curated`.
