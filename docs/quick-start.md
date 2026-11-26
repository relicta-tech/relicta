# Quick Start Guide

Set up Relicta, review a release plan, and follow the release lifecycle. Publishing to a hosted
platform requires a configured publisher and any credentials that platform needs.

## Prerequisites

- Git repository with at least one commit
- [Conventional Commits](https://www.conventionalcommits.org/) (recommended) or any commit history
- Go 1.25 or newer for the current v4.2.0 `go install`; v5 requires Go 1.26 or newer

## Installation

Choose your preferred method:

```bash
# Homebrew (Linux only; do not use on macOS until the tap Formula is migrated)
brew install relicta-tech/tap/relicta

# Go install (current published release)
go install github.com/relicta-tech/relicta/v4/cmd/relicta@v4.2.0

# Container (4.2.0 is available; see the production verification note below)
export RELICTA_VERSION="${RELICTA_VERSION:-4.2.0}"
docker run --rm \
  --entrypoint /app/relicta \
  --user "$(id -u):$(id -g)" \
  --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \
  --cap-drop=ALL \
  --security-opt=no-new-privileges \
  --pids-limit=100 \
  --memory=512m \
  --cpus=2 \
  -v "$PWD:/workspace" -w /workspace \
  "ghcr.io/relicta-tech/relicta:${RELICTA_VERSION}" version
```

The v5 module path is present in the unreleased source checkout but is not published yet. Once
v5 is released, install it with `go install github.com/relicta-tech/relicta/v5/cmd/relicta@latest`.

The `go install` command builds the CLI without the embedded Vue dashboard. The current release
configuration embeds the dashboard in release binaries; until a release built with that
configuration is published, use `make build-with-frontend` from a source checkout to include it.
The dashboard is optional when using Relicta through the CLI or MCP server.

The existing v4.2.0 tap Formula still offers unsigned Darwin archives that
Gatekeeper may block. Use `go install` or build from source on macOS.

The repository mount remains writable so Relicta can save its release state and
update files when a release requires it. The container filesystem stays
read-only; `/tmp` is the only additional writable location.

The current v4.2.0 image predates the repository's image scanning and provenance
attestation workflow. GitHub has no provenance attestation for that image digest,
so this command is a quick-start example, not a production-verified image path.
In a Trivy scan on 2026-11-26, the published `4.2.0` image had 60 HIGH/CRITICAL
findings (59 HIGH and 1 CRITICAL): 37 in Alpine packages and 23 in the Go
binary. Do not deploy that image to production; wait for a fixed release and
verify the exact published image digest before deployment.
Before using a container in production, select a release published after those
workflow controls completed and verify its image attestation as described in the
[supply-chain guide](security/supply-chain-security.md#container-images).

On Windows, run `go install` in PowerShell. If `relicta` is not found afterward, add Go's
bin directory to the current session and verify the install:

```powershell
$goBin = Join-Path (go env GOPATH) "bin"
$env:Path = "$env:Path;$goBin"
relicta version
```

### Verified Windows binary (amd64)

The published v4.2.0 release includes a Windows amd64 archive. This pinned example verifies the
signed checksum manifest, the archive digest, and GitHub build provenance before extracting the
binary. Install [Cosign](https://docs.sigstore.dev/cosign/system_config/installation/) and the
[GitHub CLI](https://cli.github.com/) first; authenticate `gh` if it asks.

```powershell
$ErrorActionPreference = 'Stop'
$version = 'v4.2.0'
$archive = 'relicta_windows_amd64.zip'
$base = "https://github.com/relicta-tech/relicta/releases/download/$version"
$temp = Join-Path $env:TEMP "relicta-$version"
New-Item -ItemType Directory -Path $temp -Force | Out-Null
$archivePath = Join-Path $temp $archive
$checksumsPath = Join-Path $temp 'checksums.txt'
$bundlePath = Join-Path $temp 'checksums.txt.sigstore.json'

Invoke-WebRequest "$base/$archive" -OutFile $archivePath
Invoke-WebRequest "$base/checksums.txt" -OutFile $checksumsPath
Invoke-WebRequest "$base/checksums.txt.sigstore.json" -OutFile $bundlePath

$identity = '^https://github\.com/relicta-tech/relicta/\.github/workflows/release\.yaml@refs/tags/' + [regex]::Escape($version) + '$'
& cosign verify-blob --bundle $bundlePath --certificate-identity-regexp $identity `
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' $checksumsPath
if ($LASTEXITCODE -ne 0) { throw 'Checksum manifest signature verification failed.' }

$entries = @(
  Get-Content $checksumsPath | ForEach-Object {
    if ($_ -match '^([a-fA-F0-9]{64})\s+\*?(.+)$') {
      [PSCustomObject]@{ Hash = $Matches[1].ToLowerInvariant(); Name = $Matches[2].Trim() }
    }
  } | Where-Object { $_.Name -eq $archive }
)
if ($entries.Count -ne 1) { throw "Expected one checksum entry for $archive; found $($entries.Count)." }
$actual = (Get-FileHash $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne $entries[0].Hash) { throw 'Archive checksum does not match the signed manifest.' }

& gh attestation verify $archivePath --repo relicta-tech/relicta `
  --signer-workflow relicta-tech/relicta/.github/workflows/release.yaml `
  --source-ref "refs/tags/$version"
if ($LASTEXITCODE -ne 0) { throw 'Archive provenance verification failed.' }

$extractPath = Join-Path $temp 'extracted'
Expand-Archive -Path $archivePath -DestinationPath $extractPath -Force
$binary = Get-ChildItem $extractPath -Filter 'relicta.exe' -File -Recurse | Select-Object -First 1
if (-not $binary) { throw 'The verified archive did not contain relicta.exe.' }
$installPath = Join-Path $env:LOCALAPPDATA 'Programs\Relicta'
New-Item -ItemType Directory -Path $installPath -Force | Out-Null
Copy-Item $binary.FullName (Join-Path $installPath 'relicta.exe') -Force

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $installPath) {
  $updatedUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $installPath } else { "$userPath;$installPath" }
  [Environment]::SetEnvironmentVariable('Path', $updatedUserPath, 'User')
}
$env:Path = "$installPath;$env:Path"
& (Join-Path $installPath 'relicta.exe') version
```

This release-specific example is for Windows amd64. It requires the checksum signature and
archive provenance to name the same tag as `$version`. Check the selected release's assets and
provenance before changing `$version`; Windows ARM64 archives are not currently published.

```bash
# Direct download (Linux, x86_64 or ARM64).
# Resolves one stable release tag, then uses it for all downloads and provenance checks.
# Requires curl, cosign, the GitHub CLI (gh), and sha256sum or shasum.
set -euo pipefail
command -v cosign >/dev/null 2>&1 || { echo "Install cosign to verify the signed release checksums" >&2; exit 1; }
command -v gh >/dev/null 2>&1 || { echo "Install GitHub CLI (gh) to verify release provenance" >&2; exit 1; }
case "$(uname -s)" in
  Linux)  os=linux ;;
  *) echo "Unsupported operating system" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "Unsupported CPU architecture" >&2; exit 1 ;;
esac
asset="relicta_${os}_${arch}.tar.gz"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
version="$(gh release view --repo relicta-tech/relicta --json tagName --jq .tagName)"
if [[ ! "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "Could not resolve a stable Relicta release tag" >&2
  exit 1
fi
identity_version="${version//./\\.}"
identity="^https://github\.com/relicta-tech/relicta/\.github/workflows/release\.yaml@refs/tags/${identity_version}$"
base="https://github.com/relicta-tech/relicta/releases/download/${version}"
curl -fL --retry 3 "${base}/${asset}" -o "${tmpdir}/${asset}"
curl -fL --retry 3 "${base}/checksums.txt" -o "${tmpdir}/checksums.txt"
curl -fL --retry 3 "${base}/checksums.txt.sigstore.json" -o "${tmpdir}/checksums.txt.sigstore.json"
cosign verify-blob --bundle "${tmpdir}/checksums.txt.sigstore.json" \
  --certificate-identity-regexp "${identity}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  "${tmpdir}/checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  awk -v asset="${asset}" '$2 == asset { print; found++ } END { if (found != 1) exit 1 }' "${tmpdir}/checksums.txt" | (cd "${tmpdir}" && sha256sum -c -)
elif command -v shasum >/dev/null 2>&1; then
  awk -v asset="${asset}" '$2 == asset { print; found++ } END { if (found != 1) exit 1 }' "${tmpdir}/checksums.txt" | (cd "${tmpdir}" && shasum -a 256 -c -)
else
  echo "Install sha256sum or shasum to verify the download" >&2
  exit 1
fi
gh attestation verify "${tmpdir}/${asset}" \
  --repo relicta-tech/relicta \
  --signer-workflow relicta-tech/relicta/.github/workflows/release.yaml \
  --source-ref "refs/tags/${version}"
tar -xzf "${tmpdir}/${asset}" -C "${tmpdir}" relicta
sudo install -m 0755 "${tmpdir}/relicta" /usr/local/bin/relicta
```

Verify installation:

```bash
relicta version
```

## Initialize Your Project

Run `relicta init` with no flags. This is **zero-config quick mode**: Relicta detects
your project from the git remote and manifests (`go.mod`, `package.json`, `Cargo.toml`,
etc.), writes a `.relicta.yaml` with sensible defaults, and prints your next steps — no
prompts to answer.

```bash
cd your-project
relicta init
```

That's it — you're ready to release. A few useful variations:

```bash
relicta init --force    # Overwrite an existing .relicta.yaml
relicta init --format json   # Write .relicta.json instead of YAML
```

Prefer an explanatory, step-by-step walkthrough? Opt in to the interactive setup wizard:

```bash
relicta init --guided
```

The guided wizard asks about your project type, AI provider, and plugins, and previews the
config before saving. See the [Wizard Guide](wizard-guide.md) for the full walkthrough.

## Your First Release

### Option 1: One Command

Preview the proposed version and configured version-file changes. The dry run checks release
prerequisites, including the actor budget, allowed branch, clean working tree, and branch freshness,
but does not write files, generate release notes, or evaluate governance. A live run can write files,
create a local tag, execute publisher plugins, and push if configured. Run it only when you intend
to execute that release and review its generated notes before approving:

```bash
cd your-project

# Preview what will happen
relicta release --dry-run

# Execute only after reviewing the preview and intending to release
relicta release
```

This runs the complete workflow: analyze → version → notes → approve → publish. By default it
prompts for approval, but publishing plugins and configured Git pushes can still have external
effects. In CI, use the `plan` command for a read-only preview unless unattended release approval
is an explicit policy; `--yes --ci` bypasses the approval prompt.

### Option 2: Step by Step (Learn the Flow)

For more control and understanding, run each step:

```bash
# 1. Initialize Relicta (zero-config quick mode by default)
relicta init

# 2. Analyze commits and plan the release
relicta plan

# 3. Calculate and apply the version
relicta bump

# 4. Generate release notes
relicta notes

# 5. Review and approve
relicta approve

# 6. Publish the release
relicta publish
```

## Understanding the Workflow

Relicta uses a state machine to track releases through their lifecycle:

```
Draft → Planned → Versioned → NotesReady → Approved → Published
                                    ↓
                              (or) Canceled/Failed
```

Each command transitions the release to the next state:

| Command | Transition | What Happens |
|---------|------------|--------------|
| `plan` | Draft → Planned | Analyzes commits, calculates risk score |
| `bump` | Planned → Versioned | Sets the selected version and writes configured version files |
| `notes` | Versioned → NotesReady | Generates changelog and release notes |
| `approve` | NotesReady → Approved | Governance check, human approval |
| `publish` | Approved → Published | Runs hooks and plugins; creates the configured local tag and pushes it only when enabled |

### Write customer-facing notes with your coding agent

If commit messages are too technical for customers, use the MCP workflow in the agent session
where your product context already lives. Ask Relicta for a read-only plan recommendation, draft
notes from its listed changes, then save the exact draft with `relicta_notes` after planning and
bumping. Review every claim before approval: Relicta preserves agent-authored text but does not
fact-check it. See the [MCP release-notes workflow](mcp.md#draft-customer-facing-release-notes-in-your-agent-session).

By default, Relicta creates the release tag locally and does not push it to your remote. Set
`versioning.git_push: true` only when you want Relicta to push tags; pushing a tag may trigger
your repository's own release workflow. A GitHub Release is created only when the GitHub plugin
is enabled and has a token with the required repository permissions. If another workflow already
creates GitHub Releases on tag push, configure one publisher to avoid duplicate releases.

## Configuration

For a minimal manual setup, create a `.relicta.yaml` with these settings:

```yaml
# Minimal config - Relicta works with sensible defaults
versioning:
  strategy: conventional  # Set a one-off version with `relicta bump --version`
  tag_prefix: v
  git_tag: true
  git_push: false  # Local tag by default; push only when your workflow intends it

changelog:
  file: CHANGELOG.md
  format: keep-a-changelog
```

For automatic setup, run `relicta init`. Its default quick mode detects repository details and
writes project defaults with governance enabled; use `relicta init --guided` for the interactive
setup that explains the available options.

### Enable AI-Powered Release Notes (Optional)

AI is disabled by default. Without it, Relicta generates deterministic notes from the
classified changes; it does not turn commit messages into polished customer prose.

```yaml
ai:
  enabled: true
  provider: openai  # openai, anthropic, gemini, azure, ollama
  model: gpt-6.2-sol
  tone: professional
  audience: developers
```

Set your API key:

```bash
export OPENAI_API_KEY="your-key"
```

Then generate notes:

```bash
relicta notes -a public  # Select the audience for configured AI notes
relicta notes            # AI generation is controlled by ai.enabled in .relicta.yaml
```

### Enable the GitHub Plugin

Plugins are unsigned and run as separate processes. Review `relicta plugin sandbox-status`
before enabling one. Loading an enabled plugin requires the explicit
`--allow-untrusted-plugins` flag on Linux, macOS, and Windows; this acknowledges the current
enforcement limits and does not sandbox or verify the plugin's publisher.

```yaml
plugins:
  - name: github
    enabled: true
    config:
      draft: false
```

Set your token:

```bash
export GITHUB_TOKEN="your-token"
```

Pass `--allow-untrusted-plugins` to the Relicta command that loads the plugin, for example:

```bash
relicta publish --allow-untrusted-plugins
```

## Governance (CGP)

The Change Governance Protocol helps you decide **what should ship** by assessing risk and enforcing policies.

### Enable Governance

Add to your config:

```yaml
governance:
  enabled: true
  policy_dir: .relicta/policies
```

Cognitive backends (Chronos enabled by default; Mnemos adapter migration pending):

```yaml
mnemos:
  enabled: false # Enable only for a compatible legacy server
  endpoint: http://localhost:7777
  timeout: 10s
  namespace: relicta

chronos:
  enabled: true  # Set false to opt-out
  endpoint: http://localhost:7778
  timeout: 10s
  metrics:
    - risk_score
    - release_frequency
    - lead_time
```

### Create a Policy

Create `.relicta/policies/default.policy`:

```
rule "require-approval-for-major" {
    description = "Major versions need human approval"

    when {
        bump_type == "major"
    }

    then {
        require_approval(role: "release-manager")
    }
}

rule "auto-approve-patches" {
    description = "Auto-approve low-risk patches"

    when {
        bump_type == "patch"
        risk_score < 0.3
    }

    then {
        approve()
    }
}
```

### View Risk Assessment

```bash
relicta plan -a         # Short form
relicta plan --analyze  # Long form
relicta evaluate        # Governance-only evaluation output
```

Output shows:
- Risk score (0.0 - 1.0)
- Contributing factors
- Recommended action

Test policy behavior without running a release:

```bash
relicta policy test --risk-score 0.85 --bump-type major --actor-type agent
```

See the [CGP Guide](governance.md) for full policy DSL documentation.
For optional Mnemos/Chronos setup, see [Cognitive Backends](cognitive-backends.md).

## Common Patterns

### CI/CD Auto-Release

Use this only when unattended release approval is an intentional repository policy and the
configured publisher destinations have been reviewed. `--yes` bypasses the interactive approval
prompt; governance policies still apply, and configured plugins may publish externally. For
routine CI checks, use `relicta plan` instead.

```bash
relicta release -y    # Short form
relicta release --yes --ci # Long form; CI mode is non-interactive and emits JSON
```

The `-y` / `--yes` flag auto-approves releases (subject to governance policies).

### Tag-Triggered Releases

Relicta can also run from CI after a version tag has already been pushed. Use this mode only when
your tag-triggered workflow is configured to run Relicta and you have chosen one publishing
workflow. Do not run the same release locally as well, or configure a second publisher to create
the same release:

```bash
# Push a version tag to trigger the configured workflow:
git tag v1.2.0
git push origin v1.2.0
```

In the tag-triggered workflow, check out full Git history and run:

```bash
relicta release --yes --ci
```

Relicta detects that the checked-out commit is already tagged and processes that version. If you
are releasing locally, use `relicta release` instead and let Relicta create the tag. By default it
creates the tag locally without pushing it.

### Preview Without Changes

Always available with `--dry-run`:

```bash
relicta release --dry-run
relicta bump --dry-run
```

`relicta notes` generates and saves release-note state, so review the output before
approving rather than using it as a read-only preview.

### Skip Tag Push

For local testing or when CI handles pushes:

```bash
relicta publish -P           # Short form
relicta publish --skip-push  # Long form
```

### Clean Up Stale Releases

If you have old release runs that weren't completed:

```bash
# See what would be deleted
relicta clean --dry-run

# Keep last 5 releases, delete older
relicta clean -k 5        # Short form
relicta clean --keep 5    # Long form

# Delete releases older than 30 days
relicta clean -o 30d             # Short form
relicta clean --older-than 30d   # Long form
```

## GitHub Actions

Use Go to install a pinned Relicta version for read-only CI checks. The current
[Relicta GitHub Action](https://github.com/relicta-tech/relicta-action) release warns and continues
when binary checksum verification fails, so do not use it to execute Relicta in production CI
until a fail-closed release is available.

```yaml
name: Release

on:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v6
        with:
          fetch-depth: 0
          persist-credentials: false

      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v6
        with:
          go-version: '1.26'
      - run: go install github.com/relicta-tech/relicta/v4/cmd/relicta@v4.2.0
      - run: relicta plan --dry-run --json
```

For verified binary downloads, use the signed [direct-download procedure](#installation).

## MCP Integration (AI Agents)

Relicta can give your coding agent the structured change summary, then save the
customer-facing release notes the agent writes using your product context. This
uses the agent session you already have; you do not need to configure a second
AI provider just to draft the notes.

This workflow is available in the unreleased v5 source. The current public
v4.2.0 binary does not include the agent-authored draft input; use a v5 source
build until v5 is published.

Start the server:

```bash
relicta mcp serve
```

Add to Claude Desktop's config:

```json
{
  "mcpServers": {
    "relicta": {
      "command": "relicta",
      "args": ["mcp", "serve"]
    }
  }
}
```

For a release, ask the agent to prepare customer-facing notes. It should inspect
the plan first, create the versioned release, write the draft from the plan's
recommendation, then save it for review:

1. Call `relicta_plan` with `dry_run: true` to inspect the proposed version and
   change summary without creating release state.
2. When ready, call `relicta_plan` again without `dry_run`, then
   `relicta_bump`.
3. Ask the agent to explain user-visible outcomes in plain language, omit
   internal refactors, and include only claims supported by the recommendation.
4. Save the exact text with `relicta_notes` using its `draft` field. Review the
   saved notes and governance result before approving or publishing.

For example, ask: “Write customer-facing release notes for this version using
only the plan recommendation. Explain user-visible outcomes in plain language,
omit internal refactors, and preserve any supported compatibility or security
caveats. Return only the notes.”

The agent can use product context from your conversation to translate technical
commits into customer language. Relicta stores the supplied draft with the
release; it does not fact-check agent-authored claims. If the repository changes
after planning, plan again and rewrite the draft from the updated recommendation.

See [MCP Integration Guide](mcp.md) for details.

## Troubleshooting

### "No commits found since last release"

Your repository needs at least one commit after the last tag:

```bash
git log --oneline $(git describe --tags --abbrev=0)..HEAD
```

### "No previous tag found"

For first releases, Relicta uses `0.0.0` as the base:

```bash
relicta plan  # Will suggest 0.1.0 or 1.0.0
```

### Reset a Failed Release

If a release gets stuck:

```bash
relicta cancel  # Cancel current release
relicta clean   # Clean up stale runs
relicta release # Start fresh
```

### View Current State

```bash
relicta status
```

Shows:
- Current release state
- Pending actions
- Version information

## Next Steps

- **[Configuration Reference](README.md#configuration)** - Full config options
- **[CGP Guide](governance.md)** - Policy DSL, risk scoring, approvals
- **[Cognitive Backends](cognitive-backends.md)** - Mnemos/Chronos setup and config
- **[Plugin System](PLUGINS.md)** - GitHub, npm, Slack, and more
- **[MCP Integration](mcp.md)** - AI agent support
- **[Troubleshooting](TROUBLESHOOTING.md)** - Common issues and solutions

## Command Reference

| Command | Description |
|---------|-------------|
| `relicta release` | Complete workflow in one command |
| `relicta init` | Initialize config (zero-config quick mode; `--guided` for the wizard) |
| `relicta plan` | Analyze commits and plan release |
| `relicta bump` | Apply version bump |
| `relicta notes` | Generate release notes |
| `relicta approve` | Review and approve |
| `relicta publish` | Execute the release |
| `relicta status` | View current state |
| `relicta cancel` | Cancel active release |
| `relicta clean` | Remove stale releases |
| `relicta mcp serve` | Start MCP server |

### Common Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Path to config file |
| `--verbose` | `-v` | Enable verbose output |
| `--dry-run` | | Preview without changes |
| `--yes` | `-y` | Auto-approve (approve, release) |
| _(removed)_ | | `--ai` is gone; set `ai.enabled: true` in `.relicta.yaml` instead |
| `--analyze` | `-a` | Include detailed analysis (plan) |
| `--keep` | `-k` | Keep last N runs (clean) |

Run `relicta <command> --help` for all available flags.
