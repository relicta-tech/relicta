# Troubleshooting Guide

Common issues and solutions when using Relicta.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Configuration Errors](#configuration-errors)
- [Git and Version Control](#git-and-version-control)
- [Plugin Errors](#plugin-errors)
- [GitHub Action Issues](#github-action-issues)
- [AI/OpenAI Errors](#aiopenai-errors)
- [Performance Issues](#performance-issues)

---

## Installation Issues

### "command not found: relicta"

**Cause**: Binary not in PATH or not installed.

**Solution**:

```bash
# If installed via go install, ensure GOPATH/bin is in PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Or install to /usr/local/bin
sudo cp relicta /usr/local/bin/

# Verify installation
which relicta
relicta version
```

### "permission denied"

**Cause**: Binary not executable.

**Solution**:

```bash
chmod +x relicta
# Or if installed via go install
chmod +x $(go env GOPATH)/bin/relicta
```

---

## Configuration Errors

### "config file not found"

**Cause**: No `.relicta.yaml` in current directory or `~/.config/relicta/`.

**Solution**:

```bash
# Initialize config
relicta init

# Or specify config path
relicta --config /path/to/config.yaml plan
```

### "invalid config: ..."

**Cause**: YAML syntax error or invalid configuration.

**Solution**:

```bash
# Validate config
relicta validate

# Common issues:
# - Incorrect indentation (use 2 spaces, not tabs)
# - Missing required fields
# - Invalid plugin names
```

**Example valid config**:

```yaml
versioning:
  strategy: conventional
  initial_version: "0.1.0"

changelog:
  enabled: true
  file: CHANGELOG.md
  format: conventional

plugins:
  - name: github
    enabled: true
```

### "unsupported versioning strategy"

**Cause**: `versioning.strategy` is set to a value the current release workflow does not support.

**Solution**: Use one of the supported strategies:
- `conventional` - Relicta recommends semantic version bumps from conventional commits

To choose a specific version for one release, run `relicta bump --version 2.0.0` after planning.
There is no `versioning.version` config key.

```yaml
versioning:
  strategy: conventional  # ✅ Correct
  # strategy: semantic   # ❌ Wrong
```

---

## Git and Version Control

### "no commits found since last release"

**Cause**: No new commits between current HEAD and last git tag.

**Solution**:

```bash
# Check if there are commits
git log $(git describe --tags --abbrev=0)..HEAD

# If empty, make commits before releasing
git commit -m "feat: add new feature"
```

### "no git repository found"

**Cause**: Not running in a git repository.

**Solution**:

```bash
# Initialize git repository
git init
git add .
git commit -m "feat: initial commit"
```

### "failed to parse commit message"

**Cause**: Commit doesn't follow conventional commit format.

**Solution**: Use conventional commit format:

```bash
# ✅ Correct
git commit -m "feat: add user authentication"
git commit -m "fix: resolve login bug"
git commit -m "docs: update README"

# ❌ Wrong
git commit -m "added feature"
git commit -m "fixed stuff"
```

**Conventional Commit Format**:
```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**: feat, fix, docs, style, refactor, perf, test, chore

### "BREAKING CHANGE not detected"

**Cause**: Breaking change not properly marked.

**Solution**: Include `BREAKING CHANGE:` in commit body or use `!`:

```bash
# Method 1: In commit body
git commit -m "feat: new API

BREAKING CHANGE: API endpoints changed from /api/v1 to /api/v2"

# Method 2: Use ! after type
git commit -m "feat!: redesign API"
```

---

## Plugin Errors

### "plugin 'github' not found"

**Cause**: GitHub plugin binary not installed.

**Solution**:

For a GitHub Actions plan, install Relicta from its pinned Go module version:
```yaml
permissions:
  contents: read

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

For local/manual use, install the plugin from the configured registry and enable it:
```bash
relicta plugin list --available
relicta plugin install github
relicta plugin enable github
relicta plugin info github
```

Relicta plugins are unsigned. Before loading an enabled plugin, review
`relicta plugin sandbox-status` and the plugin source. Then pass
`--allow-untrusted-plugins` to the Relicta command that loads it, such as
`relicta publish --allow-untrusted-plugins`. This flag records your trust
decision; it does not verify the plugin publisher or provide a complete sandbox.

### "plugin failed: missing GITHUB_TOKEN"

**Cause**: GitHub token not set.

**Solution**:

```bash
# Set environment variable
export GITHUB_TOKEN="ghp_your_token_here"

# Or in GitHub Actions (automatic)
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### "plugin failed: missing SLACK_WEBHOOK_URL"

**Cause**: Slack webhook URL not configured.

**Solution**:

```bash
# Set webhook URL
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"

# Verify
echo $SLACK_WEBHOOK_URL
```

### "plugin communication failed"

**Cause**: Plugin crashed or binary incompatible.

**Solution**:

```bash
# Check plugin binary
ls -lh ~/.relicta/plugins/relicta-plugin-github

# Test plugin directly
~/.relicta/plugins/relicta-plugin-github --version

# Check logs
cat .relicta/logs/plugin-github.log

# Reinstall plugin
rm ~/.relicta/plugins/relicta-plugin-github
# Download fresh copy (see "plugin not found" solution)
```

---

## GitHub Action Issues

### "Action failed: binary not found"

**Cause**: Action failed to download Relicta binary.

**Solution**: Check GitHub Actions logs. For a read-only plan, install the pinned Go module rather
than the current Relicta Action release, which warns and continues on checksum errors:

```yaml
permissions:
  contents: read

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

### "Action failed: checksum mismatch"

**Cause**: Downloaded binary checksum doesn't match expected value.

**Solution**: Stop the workflow and verify the published release archive and signed checksum
manifest using the [direct-download procedure](quick-start.md#installation). The current v2
Action warns and continues when checksum verification fails, so a green Action step does not
prove the downloaded binary was authenticated. For production, use the direct-download flow
with Cosign signature and GitHub provenance verification, or wait for a fail-closed Action.

### "Permission denied: contents write"

**Cause**: A publishing workflow's `GITHUB_TOKEN` lacks permission to create a GitHub Release.
The read-only `relicta plan` examples above do not need write permission.

**Solution**: Scope `contents: write` to the publishing job only. Install Relicta from a pinned Go
module version or verify the binary with the signed direct-download flow before granting it write
permission. The current Relicta Action release warns and continues when checksum verification
fails; wait for a fail-closed Action release before using it in a publishing workflow.

### "No commits found"

**Cause**: Shallow checkout doesn't include full git history.

**Solution**:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0  # ✅ Fetch full history
```

---

## AI/OpenAI Errors

### "OpenAI API key not found"

**Cause**: AI features are configured to use OpenAI, but the `OPENAI_API_KEY`
environment variable is not available to Relicta.

**Solution**:

```bash
# Set API key
export OPENAI_API_KEY="sk-..."

# Or in GitHub Actions
env:
  OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

**Note**: AI features are optional. Relicta works without AI:
```bash
# Generate notes without AI
relicta notes  # Uses deterministic notes based on the classified changes
```

If you want to disable provider-generated notes, leave AI disabled or set the
top-level `ai.enabled` option in `.relicta.yaml`:

```yaml
ai:
  enabled: false
```

### "OpenAI rate limit exceeded"

**Cause**: Too many API requests.

**Solution**:
1. Wait a few minutes and retry
2. Set `ai.enabled: false` in `.relicta.yaml` to use deterministic notes

### "OpenAI API error: ..."

**Cause**: Various API issues.

**Solution**:

```bash
# Disable AI and use basic changelog
relicta notes
```

Set the top-level setting in `.relicta.yaml`:

```yaml
ai:
  enabled: false
```

---

## Performance Issues

### "relicta is slow"

**Possible causes and solutions**:

1. **Large git history**
   ```bash
   # Limit commit analysis depth
   relicta plan --max-commits 100
   ```

2. **Many files to process**
   ```bash
   # Use .releaseignore to exclude files
   echo "node_modules/" >> .releaseignore
   echo "dist/" >> .releaseignore
   ```

3. **Slow plugin execution**
   ```bash
   # Disable unused plugins
   plugins:
     - name: slack
       enabled: false  # Temporarily disable
   ```

### "GitHub API rate limit"

**Cause**: Too many GitHub API calls.

**Solution**:

```bash
# Use authentication (higher rate limits)
export GITHUB_TOKEN="ghp_..."

# Check rate limit status
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/rate_limit
```

---

## Debugging

### Enable Debug Logging

```bash
# Set debug log level
export RELICTA_LOG_LEVEL=debug

# Run command
relicta plan
```

### Check Logs

```bash
# View logs
cat .relicta/logs/relicta.log

# View plugin logs
cat .relicta/logs/plugin-*.log
```

### Verbose Output

```bash
# Use verbose flag
relicta --verbose plan
```

### Dry Run Mode

Always test with dry run first:

```bash
# Test without making changes
relicta publish --dry-run
```

---

## Getting Help

### Check Documentation

- [Main README](../README.md)
- [Plugin Guide](PLUGINS.md)
- [Configuration Examples](../examples/)

### Search Issues

Check if your issue is already reported:
- [GitHub Issues](https://github.com/relicta-tech/relicta/issues)

### Report a Bug

If you found a bug, please report it with:

1. **Environment info**:
   ```bash
   relicta version
   go version  # If relevant
   git --version
   uname -a  # Linux/macOS
   ```

2. **Configuration** (sanitize secrets):
   ```yaml
   # Your .relicta.yaml
   ```

3. **Commands run**:
   ```bash
   relicta plan --verbose
   ```

4. **Error messages**:
   ```
   Full error output
   ```

5. **Logs**:
   ```bash
   cat .relicta/logs/relicta.log
   ```

### Ask Questions

- [GitHub Discussions](https://github.com/relicta-tech/relicta/discussions)
- [Issues](https://github.com/relicta-tech/relicta/issues/new)

---

## Common Workflows

### Reset Release State

If you need to start over:

```bash
# Remove release state
rm -rf .relicta/state/

# Start fresh
relicta plan
```

### Skip Plugin Temporarily

Disable the plugin in the configuration used for this command:

```yaml
# In .relicta.yaml, or the file passed with --config:
plugins:
  - name: slack
    enabled: false
```

### Manual Version Override

```bash
# Set a specific version after planning
relicta bump --version 2.0.0
```

### Test in CI/CD

```yaml
permissions:
  contents: read

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

---

## Best Practices

1. **Always use `--dry-run` first**
   ```bash
   relicta publish --dry-run
   ```

2. **Validate config before committing**
   ```bash
   relicta validate
   ```

3. **Keep plugins up to date**
   ```bash
   # Check for updates
   relicta plugin list
   ```

4. **Use conventional commits consistently**
   ```bash
   # Set up commit message template
   git config commit.template .gitmessage
   ```

5. **Test in a branch first**
   ```bash
   git checkout -b test-release
   relicta publish --dry-run
   ```

6. **Monitor logs in production**
   ```bash
   tail -f .relicta/logs/relicta.log
   ```
