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

### "unknown versioning strategy"

**Cause**: Invalid `versioning.strategy` value.

**Solution**: Use one of the supported strategies:
- `conventional` - Semantic versioning from conventional commits
- `manual` - Manually specify version

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

For GitHub Actions:
```yaml
# The action handles this automatically
- uses: relicta-tech/relicta-action@v1
```

For local/manual use:
```bash
# Download plugin binary
mkdir -p ~/.relicta/plugins
cd ~/.relicta/plugins

# Linux x86_64
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/github_linux_x86_64 \
  -o relicta-plugin-github
chmod +x relicta-plugin-github

# macOS arm64
curl -L https://github.com/relicta-tech/relicta/releases/latest/download/github_darwin_aarch64 \
  -o relicta-plugin-github
chmod +x relicta-plugin-github
```

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

**Solution**: Check GitHub Actions logs:

```yaml
# Enable debug logging
- uses: relicta-tech/relicta-action@v1
  env:
    ACTIONS_STEP_DEBUG: true
```

### "Action failed: checksum mismatch"

**Cause**: Downloaded binary checksum doesn't match expected value.

**Solution**:
1. Retry the workflow (may be temporary download issue)
2. Check if release assets are corrupted
3. Report issue if persists

### "Permission denied: contents write"

**Cause**: Insufficient permissions to create releases.

**Solution**:

```yaml
jobs:
  release:
    permissions:
      contents: write  # ✅ Add this
    steps:
      - uses: relicta-tech/relicta-action@v1
```

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

**Cause**: `OPENAI_API_KEY` environment variable not set.

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
relicta notes  # Uses conventional commit format only
```

### "OpenAI rate limit exceeded"

**Cause**: Too many API requests.

**Solution**:
1. Wait a few minutes and retry
2. Use `--ai=false` to disable AI features
3. Upgrade OpenAI API plan for higher limits

### "OpenAI API error: ..."

**Cause**: Various API issues.

**Solution**:

```bash
# Disable AI and use basic changelog
relicta notes --ai=false

# Or set in config
changelog:
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

```bash
# Disable in config
plugins:
  - name: slack
    enabled: false

# Or use environment variable
RELICTA_PLUGINS="" relicta publish
```

### Manual Version Override

```bash
# Set specific version
relicta bump --version 2.0.0

# Or in config
versioning:
  strategy: manual
  version: "2.0.0"
```

### Test in CI/CD

```yaml
# Dry run in CI
- uses: relicta-tech/relicta-action@v1
  with:
    dry-run: true  # Test without creating release
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
