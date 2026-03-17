# Skill Publishing Checklist

Use this checklist to publish `skills/relicta-release-governance` to channels where others can discover and install it.

## 1. Prepare the Skill Package

- Confirm required files exist:
  - `skills/relicta-release-governance/SKILL.md`
  - `skills/relicta-release-governance/agents/openai.yaml`
- Verify examples referenced by the skill are present and valid.
- Keep the skill focused on operational steps (avoid long conceptual docs inside skill folder).

## 2. Validate Locally

Run from repo root:

```bash
go test ./...
go run ./cmd/relicta policy validate --file examples/policies/starter.policy
go run ./cmd/relicta policy test --file examples/policies/starter.policy --matrix examples/policies/policy-matrix.yaml --fail-on-blocked --json
```

## 3. Publish in This Repository

- Commit the skill directory and supporting docs/examples.
- Open a PR with:
  - Skill purpose and trigger phrases.
  - Sample prompts and expected outcomes.
  - Validation commands and outputs.
- Merge to `main`.

## 4. Publish to Public Discovery Channels

Use the channels your users actually search:

1. GitHub repository discovery:
- Tag release notes with `skill`, `codex`, `gpt`, `mcp`, `relicta`.
- Add a README section that links directly to `skills/relicta-release-governance/SKILL.md`.

2. Internal/curated skill catalogs:
- Submit the skill folder (or repository URL) with a concise description and usage examples.
- Include the exact install path and version/tag.

3. GPT-facing distribution:
- Convert the skill flow into your GPT/tool instructions package.
- Reuse the same command examples and safety gates from `SKILL.md` to keep behavior aligned.

## 5. Operate and Maintain

- Treat `SKILL.md` changes as versioned API changes for agent behavior.
- Re-run validation after every change touching:
  - Relicta commands used in the skill
  - Policy fixtures under `examples/policies/`
  - MCP transport/command flags
- Keep `agents/openai.yaml` in sync whenever `SKILL.md` scope changes.
