#!/usr/bin/env python3
"""Guard the release-governance skill against unsafe quick-start regressions."""

from pathlib import Path
import re
import sys


SKILL = Path(__file__).resolve().parents[1] / "skills/relicta-release-governance/SKILL.md"
RELEASE_CLI = Path(__file__).resolve().parents[1] / "internal/cli/release.go"
text = SKILL.read_text(encoding="utf-8")
release_source = RELEASE_CLI.read_text(encoding="utf-8")
quick_start = text.split("## Quick Start", 1)[1].split("## Workflow", 1)[0]
authorized_section = text.split("## Step 5: Approval and Publish (Explicit Authorization Required)", 1)[1].split("## Step 6:", 1)[0]
errors: list[str] = []

if "relicta plan --analyze --dry-run" not in quick_start:
    errors.append("quick start must include a read-only analysis preview")
if "relicta release --dry-run" not in quick_start:
    errors.append("quick start must include the supported full release preview")
if "explicitly asks to execute that release" not in quick_start:
    errors.append("quick start must require explicit authorization for a live release")
if "Never infer publish authorization" not in quick_start:
    errors.append("quick start must distinguish planning from publish authorization")
if "does not generate release notes or evaluate governance" not in text:
    errors.append("skill must state release dry-run limitations")

version_write = release_source.split("// Write the configured manifests", 1)[1].split(
    'fmt.Printf("  Version:', 1
)[0]
if not (
    "if dryRun" in version_write
    and "reportVersionFilePlan(ctx, app, bumpOutput.Version)" in version_write
    and "applyVersionFiles(ctx, app, bumpOutput.Version)" in version_write
):
    errors.append("release dry-run must plan version-file changes without calling the writer")

for command in ("relicta init", "relicta bump", "relicta publish", "relicta release"):
    for match in re.finditer(rf"(?m)^\s*{re.escape(command)}(?:\s|$)(.*)$", quick_start):
        if "dry-run" not in match.group(1):
            errors.append(f"unsafe live command in quick start: {match.group(0).strip()}")

for command in ("relicta approve", "relicta publish", "relicta release"):
    for match in re.finditer(rf"(?m)^\s*{re.escape(command)}(?:\s|$)(.*)$", text):
        if "dry-run" not in match.group(1) and match.group(0).strip() not in authorized_section:
            errors.append(f"live command outside the authorized release section: {match.group(0).strip()}")

if errors:
    print("Release skill safety check failed:", file=sys.stderr)
    for error in errors:
        print(f"- {error}", file=sys.stderr)
    raise SystemExit(1)

print("Release skill safety check passed")
