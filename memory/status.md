---
updated: 2026-06-13
---
## Current State
Six PRs open from a full Tier A + Tier B sweep off v4.1.0, none merged yet.
Governance stack: #158 attribution → #159 earned-trust → #163 identity-grants
(must merge in that order). Independent off main: #160 CLI exit-codes, #161 MCP
split, #162 darwin sandbox-exec. All build/test/lint clean. This branch
(feat/identity-trust) holds the most complete memory snapshot.

## Last Session Summary
Scanned codebase (3 agents), found the core gap: attribution detector built but
never wired. Shipped attribution governance + earned trust + identity grants +
CLI exit-code tests + MCP decomposition + darwin sandbox-exec confinement, each
as its own PR with tests and docs.

## Next Session Should
Review and merge the 6 PRs in dependency order (stack first: #158→#159→#163,
then #160/#161/#162 anytime). Rebase the stack if any base shifts on merge.

## Blocked / Waiting
- PRs awaiting review/merge (#158–#163).
- Pre-existing macOS-only git test flake (TestGetRepositoryRoot/Info, /private/var
  vs /var TempDir symlink) — unrelated, not ours.
- memory/ diverges across the stacked branches; reconcile after merges land.
