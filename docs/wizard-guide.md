# Setup Wizard Guide

The Relicta setup wizard provides an interactive, visual setup experience that walks you through configuration step by step. This guide covers how to use the wizard and customize your configuration.

> **The wizard is opt-in.** A bare `relicta init` runs **zero-config quick mode** — it
> detects your project from the git remote and manifests, writes `.relicta.yaml` with
> sensible defaults, and asks no questions. Reach for the wizard only when you want the
> explanatory, guided experience. See the [Quick Start](quick-start.md) for the default path.

## Quick Start

Run the wizard to create your configuration:

```bash
relicta init --guided
```

> `--interactive` (`-i`) still works as a **deprecated alias** for `--guided`.

The wizard will guide you through 8 steps:
1. **Welcome** - Introduction and overview
2. **Detection** - Automatic project analysis
3. **Project Type** - Confirm or select project type
4. **Template** - Choose configuration template
5. **Questions** - Answer template-specific questions
6. **AI Configuration** - Set up AI provider (optional)
7. **Review** - Preview generated configuration
8. **Success** - Configuration created with next steps

---

## Features

### 🎯 Intelligent Detection
- Automatically scans your project for language indicators
- Detects frameworks, build tools, and package managers
- Identifies project type with confidence scoring
- Suggests best-matching template

### 📦 10 Production Templates
Pre-configured templates for common project types:
- Open Source (Go, Node.js, Python, Rust)
- SaaS (Web App, API Service)
- CLI Tools
- Mobile Apps
- Containers
- Monorepos

### 🎨 Beautiful Terminal UI
- Built with Charmbracelet Bubbletea
- Keyboard navigation and shortcuts
- Real-time validation
- Live configuration preview

### ⚡ Smart Defaults
- Auto-populated from detected project info
- Git remote detection
- Branch name detection
- Conventional commit patterns

---

## Wizard Flow

### 1. Welcome Screen

```
┌─────────────────────────────────────────────┐
│                                             │
│    🚀 Relicta Setup Wizard            │
│                                             │
│    Let's set up your release automation    │
│    in just a few steps!                     │
│                                             │
│    Press Enter to continue                  │
│    Press q to quit                          │
│                                             │
└─────────────────────────────────────────────┘
```

**Actions:**
- **Enter** - Start wizard
- **q** - Exit wizard

---

### 2. Detection Screen

```
┌─────────────────────────────────────────────┐
│                                             │
│    🔍 Analyzing your project...             │
│                                             │
│    ⠋ Scanning files                         │
│    ✓ Detected: Go                           │
│    ✓ Found: go.mod                          │
│    ✓ Git remote: github.com/user/repo      │
│                                             │
└─────────────────────────────────────────────┘
```

The wizard automatically detects:

| Detection | What It Finds |
|-----------|---------------|
| **Language** | Go, JavaScript, Python, Rust, Java, etc. |
| **Framework** | React, Next.js, Django, FastAPI, etc. |
| **Platform** | Docker, Kubernetes, Mobile, Web |
| **Build Tools** | Make, npm, Cargo, Maven, Gradle |
| **Git Info** | Remote URL, default branch, current branch |

**Detection Indicators:**

```
Go Projects:         go.mod, *.go files, go.sum
Node.js Projects:    package.json, package-lock.json, node_modules/
Python Projects:     setup.py, requirements.txt, pyproject.toml
Rust Projects:       Cargo.toml, Cargo.lock, src/main.rs
Docker Projects:     Dockerfile, docker-compose.yml
Mobile Projects:     ios/, android/, flutter/, react-native/
```

---

### 3. Project Type Selection

```
┌─────────────────────────────────────────────┐
│                                             │
│    📦 Select Project Type                   │
│                                             │
│    Based on detection: Go Open Source      │
│                                             │
│    > Open Source Library/Tool              │
│      SaaS / Web Application                │
│      API Service / Backend                 │
│      CLI Application                        │
│      Mobile Application                     │
│      Container / Docker Image              │
│      Monorepo                               │
│                                             │
│    ↑↓ Navigate  Enter Select  q Quit       │
└─────────────────────────────────────────────┘
```

**Project Types:**

| Type | Best For | Examples |
|------|----------|----------|
| **Open Source** | Public libraries, tools, frameworks | React, Go libraries, CLI tools |
| **SaaS** | Web applications with users | Gmail, Notion, Slack |
| **API Service** | Backend APIs, microservices | REST APIs, GraphQL servers |
| **CLI Application** | Command-line tools | Git, Docker CLI, kubectl |
| **Mobile Application** | iOS, Android, React Native apps | Instagram, WhatsApp |
| **Container** | Docker images, Kubernetes apps | Nginx, PostgreSQL images |
| **Monorepo** | Multiple packages in one repo | Turborepo, Nx workspaces |

**Selection Tips:**
- ✅ Choose based on **primary use case**, not just technology
- ✅ Open Source projects benefit from GitHub releases and community notifications
- ✅ SaaS projects need changelog tracking for users
- ✅ API services focus on version compatibility and breaking changes

---

### 4. Template Selection

```
┌─────────────────────────────────────────────┐
│                                             │
│    📋 Choose Configuration Template         │
│                                             │
│    > Go Open Source Library (Recommended)  │
│      Go CLI Application                     │
│      Go API Service                         │
│      Node.js Open Source Package           │
│      Python Package (PyPI)                  │
│      Rust Crate (crates.io)                │
│      SaaS Web Application                   │
│      Container Image (Docker)               │
│      Monorepo (Lerna/Turborepo)            │
│      Custom / Minimal                       │
│                                             │
│    ↑↓ Navigate  Enter Select  q Quit       │
└─────────────────────────────────────────────┘
```

**Available Templates:**

#### Open Source Templates

**1. Go Open Source Library**
- **Plugins:** GitHub, Homebrew (optional), Docker (optional)
- **Versioning:** Conventional commits with git tags
- **Changelog:** Auto-generated with breaking changes
- **Best For:** Go libraries, CLI tools, frameworks
- **Example Projects:** Cobra, Viper, Hugo

**2. Node.js Open Source Package**
- **Plugins:** GitHub, npm
- **Versioning:** Semver with package.json updates
- **Changelog:** Conventional changelog format
- **Best For:** JavaScript/TypeScript libraries, React components
- **Example Projects:** React, Vue, Express

**3. Python Package (PyPI)**
- **Plugins:** GitHub, PyPI
- **Versioning:** PEP 440 versioning
- **Changelog:** ReStructuredText format
- **Best For:** Python libraries, Django apps, data science tools
- **Example Projects:** Flask, Pandas, NumPy

**4. Rust Crate (crates.io)**
- **Plugins:** GitHub, Cargo (crates.io)
- **Versioning:** Cargo.toml updates
- **Changelog:** Keep a Changelog format
- **Best For:** Rust libraries and applications
- **Example Projects:** Tokio, Serde, Actix

#### SaaS / Commercial Templates

**5. SaaS Web Application**
- **Plugins:** Slack, Discord, internal changelog
- **Versioning:** Marketing versions (v1.2, v2.0)
- **Changelog:** User-facing feature announcements
- **Best For:** Web apps with end users
- **Example Projects:** Notion, Linear, Superhuman

**6. API Service / Backend**
- **Plugins:** Slack, API documentation updates
- **Versioning:** API versioning (v1, v2)
- **Changelog:** Breaking changes, deprecations
- **Best For:** REST APIs, GraphQL services, microservices
- **Example Projects:** Stripe API, Twilio API

#### Specialized Templates

**7. CLI Application**
- **Plugins:** GitHub, Homebrew, Snap/Apt (Linux)
- **Versioning:** Semver with update checker
- **Changelog:** Command changes, new features
- **Best For:** Developer tools, system utilities
- **Example Projects:** Git, kubectl, terraform

**8. Mobile Application**
- **Plugins:** Slack, Discord, App Store notifications
- **Versioning:** App store versions (1.2.3, build 42)
- **Changelog:** User-facing feature list
- **Best For:** iOS, Android, React Native apps
- **Example Projects:** Instagram, Spotify

**9. Container Image (Docker)**
- **Plugins:** GitHub, Docker Hub/GHCR
- **Versioning:** Image tags (latest, v1.2.3, sha-abc123)
- **Changelog:** Image updates, security patches
- **Best For:** Docker images, Kubernetes apps
- **Example Projects:** nginx, postgres, redis

**10. Monorepo (Lerna/Turborepo)**
- **Plugins:** GitHub, npm, independent versioning
- **Versioning:** Independent or fixed mode
- **Changelog:** Separate changelogs per package
- **Best For:** Multiple packages in one repo
- **Example Projects:** Babel, Jest, Next.js

---

### 5. Template Questions

Each template asks specific questions to customize the configuration:

#### Common Questions (All Templates)

```
┌─────────────────────────────────────────────┐
│                                             │
│    📝 Configuration Questions               │
│                                             │
│    Project Name:                            │
│    ┌─────────────────────────────────────┐ │
│    │ relicta                       │ │
│    └─────────────────────────────────────┘ │
│                                             │
│    Repository URL:                          │
│    ┌─────────────────────────────────────┐ │
│    │ github.com/user/relicta       │ │
│    └─────────────────────────────────────┘ │
│                                             │
│    Default Branch:                          │
│    ┌─────────────────────────────────────┐ │
│    │ main                                │ │
│    └─────────────────────────────────────┘ │
│                                             │
│    ↑↓ Navigate  Enter Continue  q Quit     │
└─────────────────────────────────────────────┘
```

| Question | Description | Example |
|----------|-------------|---------|
| **Project Name** | Display name in changelogs | "Relicta" |
| **Repository URL** | Git remote URL | "github.com/user/repo" |
| **Default Branch** | Main branch name | "main" or "master" |
| **Tag Prefix** | Git tag prefix | "v" (creates v1.2.3) |
| **Sign Tags** | GPG sign git tags | Yes/No |

#### Template-Specific Questions

**Go Open Source:**
```
┌─────────────────────────────────────────────┐
│    Enable Homebrew releases?   [Yes] / No   │
│    Homebrew Tap:                             │
│    └─ user/homebrew-tap                      │
│                                              │
│    Enable GoReleaser?          [Yes] / No    │
│    Cross-compile platforms:                  │
│    └─ linux, darwin, windows                 │
└─────────────────────────────────────────────┘
```

**Node.js Open Source:**
```
┌─────────────────────────────────────────────┐
│    Publish to npm?             [Yes] / No    │
│    npm package name:                         │
│    └─ @scope/package-name                    │
│                                              │
│    Package manager:            [npm]         │
│    └─ npm, yarn, pnpm                        │
└─────────────────────────────────────────────┘
```

**SaaS Web App:**
```
┌─────────────────────────────────────────────┐
│    Notification channels:                    │
│    ☑ Slack                                   │
│    ☐ Discord                                 │
│    ☐ Microsoft Teams                         │
│                                              │
│    Changelog location:                       │
│    └─ /changelog, /releases, /updates        │
└─────────────────────────────────────────────┘
```

---

### 6. AI Configuration

```
┌─────────────────────────────────────────────┐
│                                             │
│    🤖 AI-Powered Features (Optional)        │
│                                             │
│    Enable AI for changelog generation?      │
│    > Yes (Recommended)                      │
│      No (Use templates only)                │
│                                             │
│    AI Provider:                             │
│    > OpenAI (GPT-4o)                        │
│      Anthropic (Claude)                     │
│      Google (Gemini)                        │
│      Ollama (Local)                         │
│      Azure OpenAI                           │
│                                             │
│    Model: gpt-4o                            │
│    API Key: [Set via OPENAI_API_KEY]        │
│                                             │
│    ↑↓ Navigate  Enter Continue  q Quit     │
└─────────────────────────────────────────────┘
```

**AI Provider Selection:**

| Provider | Best For | Setup Required |
|----------|----------|----------------|
| **OpenAI** | General use, best quality | API key from platform.openai.com |
| **Gemini** | Cost-effective, large context | API key from aistudio.google.com |
| **Anthropic** | Long changelogs, analysis | API key from console.anthropic.com |
| **Ollama** | Privacy, offline, free | Local installation (ollama.com) |
| **Azure OpenAI** | Enterprise, compliance | Azure subscription required |

**Configuration:**
- API keys are **never stored** in the config file
- Set via environment variables: `OPENAI_API_KEY`, `GEMINI_API_KEY`, etc.
- Wizard shows instructions for each provider
- AI can be disabled or changed later

---

### 7. Review & Preview

```
┌─────────────────────────────────────────────┐
│                                             │
│    📄 Configuration Preview                 │
│                                             │
│    ┌───────────────────────────────────┐   │
│    │ versioning:                       │   │
│    │   strategy: conventional          │   │
│    │   tag_prefix: v                   │   │
│    │   git_sign: true                  │   │
│    │                                   │   │
│    │ changelog:                        │   │
│    │   file: CHANGELOG.md              │   │
│    │   product_name: Relicta      │   │
│    │                                   │   │
│    │ ai:                               │   │
│    │   enabled: true                   │   │
│    │   provider: openai                │   │
│    │   model: gpt-4o                   │   │
│    │                                   │   │
│    │ plugins:                          │   │
│    │   - name: github                  │   │
│    │     enabled: true                 │   │
│    │   - name: homebrew                │   │
│    └───────────────────────────────────┘   │
│                                             │
│    ↑↓ Scroll  Enter Save  e Edit  q Cancel │
└─────────────────────────────────────────────┘
```

**Actions:**
- **↑↓** - Scroll through configuration
- **Enter** - Save configuration
- **e** - Edit (go back to questions)
- **q** - Cancel wizard

**The preview shows:**
- ✅ Complete YAML configuration
- ✅ Syntax highlighting
- ✅ All selected plugins
- ✅ AI configuration (with env var placeholders)
- ✅ Versioning strategy

---

### 8. Success Screen

```
┌─────────────────────────────────────────────┐
│                                             │
│    ✅ Configuration Created!                │
│                                             │
│    Created: .relicta.yaml             │
│                                             │
│    Next Steps:                              │
│                                             │
│    1. Set API keys (if using AI):           │
│       export OPENAI_API_KEY="sk-..."        │
│                                             │
│    2. Make your first release:              │
│       relicta publish                 │
│                                             │
│    3. Learn more:                           │
│       relicta --help                  │
│       docs/getting-started.md               │
│                                             │
│    Press any key to exit                    │
│                                             │
└─────────────────────────────────────────────┘
```

**Next Steps Guidance:**

The success screen shows personalized next steps based on your configuration:

- **If AI enabled:** Instructions to set API keys
- **If plugins enabled:** Plugin-specific setup (webhooks, tokens)
- **If Homebrew:** Tap creation instructions
- **If npm:** Publishing instructions

---

## Keyboard Shortcuts

### Global Shortcuts

| Key | Action |
|-----|--------|
| **↑ / k** | Move up |
| **↓ / j** | Move down |
| **Enter** | Select / Continue |
| **Esc / q** | Quit / Cancel |
| **Ctrl+C** | Force quit |
| **?** | Help (context-sensitive) |

### Screen-Specific

| Screen | Key | Action |
|--------|-----|--------|
| **Questions** | Tab | Next field |
| **Questions** | Shift+Tab | Previous field |
| **Review** | e | Edit configuration |
| **Review** | PgUp/PgDn | Scroll faster |
| **Selection** | / | Search/filter |

---

## Project Detection

### How Detection Works

The wizard scans your project directory for indicators:

```go
// Detection process:
1. Scan files in project root
2. Check for language indicators
3. Detect frameworks and tools
4. Analyze git configuration
5. Calculate confidence scores
6. Suggest best template
```

### Detection Accuracy

| Indicator | Confidence | Example |
|-----------|------------|---------|
| **go.mod + *.go files** | 95% Go | Go project |
| **package.json + node_modules** | 95% Node.js | JavaScript project |
| **Dockerfile + docker-compose.yml** | 90% Container | Docker project |
| **setup.py + requirements.txt** | 90% Python | Python project |
| **Cargo.toml + src/main.rs** | 95% Rust | Rust project |

### Manual Override

If detection is incorrect:
1. Select different project type in step 3
2. Choose appropriate template in step 4
3. Detection results are suggestions, not requirements

---

## Customizing Configuration

### After Wizard Completion

Edit `.relicta.yaml` to customize:

```yaml
# Add more plugins
plugins:
  - name: slack
    enabled: true
    config:
      webhook: ${SLACK_WEBHOOK_URL}

# Adjust AI settings
ai:
  temperature: 0.5  # More focused (less creative)
  max_tokens: 2048  # Shorter responses

# Customize changelog
changelog:
  exclude_types:
    - chore
    - docs
  include_authors: true
```

### Template Customization

You can mix-and-match from different templates:

```yaml
# Start with Go template, add npm publishing
versioning:
  strategy: conventional  # From Go template

plugins:
  - name: github         # From Go template
  - name: npm            # From Node.js template
    config:
      registry: https://registry.npmjs.org
```

---

## Advanced Usage

### Non-Interactive / CI Mode

The wizard is for humans. In CI/CD or any non-interactive setup, skip it entirely — a bare
`relicta init` already runs zero-config quick mode and never prompts:

```bash
# Generate default configuration from detected project info (no prompts)
relicta init

# Write to a different format
relicta init --format json

# Overwrite an existing config file
relicta init --force
```

For repeatable, reviewable setups, the recommended approach is to **commit a pre-made
`.relicta.yaml`** to your repository. Generate it once (with `relicta init` or
`relicta init --guided`), tune it by hand, and check it in — CI then uses the committed
config directly with no init step required.

### Configuration Validation

Validate after manual edits:

```bash
relicta config validate

# Output:
# ✓ Configuration is valid
# ✓ All required plugins found
# ⚠ Warning: OPENAI_API_KEY not set
```

---

## Troubleshooting

### Wizard Crashes or Freezes

**Issue:** Wizard not responding

**Solutions:**
1. Press **Ctrl+C** to force quit
2. Check terminal size (minimum 80x24)
3. Update to latest version:
   ```bash
   relicta version
   brew upgrade relicta
   ```
4. Run with debug logging:
   ```bash
   RELICTA_DEBUG=1 relicta init --guided
   ```

### Detection Incorrect

**Issue:** Wrong project type detected

**Solutions:**
1. Manually select correct type in step 3
2. Choose appropriate template in step 4
3. Detection is a suggestion - override as needed
4. File issue if consistently wrong: https://github.com/relicta-tech/relicta/issues

### Configuration Not Created

**Issue:** Wizard completes but no config file

**Solutions:**
1. Check file permissions in current directory
2. Verify not running in read-only directory
3. Check if config already exists (wizard won't overwrite)
4. Use custom path:
   ```bash
   relicta init --config /path/to/config.yaml
   ```

### AI Setup Fails

**Issue:** AI configuration not working

**Solutions:**
1. Verify API key is set:
   ```bash
   echo $OPENAI_API_KEY
   ```
2. Test API key manually:
   ```bash
   curl https://api.openai.com/v1/models \
     -H "Authorization: Bearer $OPENAI_API_KEY"
   ```
3. Check provider-specific docs: `docs/ai-providers.md`
4. Try dry-run to test:
   ```bash
   relicta notes --dry-run
   ```

### Template Questions Unclear

**Issue:** Don't understand what a question asks

**Solutions:**
1. Press **?** for context help
2. Use default value (pre-filled)
3. Skip optional questions
4. Edit configuration file later
5. Refer to template examples in `docs/examples/`

---

## Examples

### Example 1: Go CLI Tool

**Detection:**
```
Detected: Go
Found: go.mod, main.go, Makefile
Confidence: 95%
Suggested: Go CLI Application
```

**Questions:**
```
Project Name: mytool
Repository URL: github.com/user/mytool
Enable Homebrew: Yes
Homebrew Tap: user/homebrew-mytool
Enable GoReleaser: Yes
Platforms: linux, darwin, windows
```

**Result:**
```yaml
versioning:
  strategy: conventional
  tag_prefix: v

plugins:
  - name: github
  - name: homebrew
    config:
      tap: user/homebrew-mytool
  - name: goreleaser
```

---

### Example 2: Node.js Package

**Detection:**
```
Detected: Node.js
Found: package.json, src/, tsconfig.json
Confidence: 90%
Suggested: Node.js Open Source
```

**Questions:**
```
Project Name: @myorg/awesome-lib
Publish to npm: Yes
Package manager: pnpm
Scope: @myorg
```

**Result:**
```yaml
versioning:
  strategy: conventional

plugins:
  - name: github
  - name: npm
    config:
      registry: https://registry.npmjs.org
      package_manager: pnpm
```

---

### Example 3: SaaS Web App

**Detection:**
```
Detected: React
Found: package.json, next.config.js, vercel.json
Confidence: 85%
Suggested: SaaS Web Application
```

**Questions:**
```
Project Name: MyApp
Notification channels: Slack, Discord
Changelog location: /changelog
User-facing notes: Yes
```

**Result:**
```yaml
versioning:
  strategy: marketing

changelog:
  file: public/changelog.md
  user_facing: true

plugins:
  - name: slack
  - name: discord
```

---

## Best Practices

### 1. Use AI for Better Changelogs

✅ **Enable AI** unless you have privacy/cost concerns
- Dramatically improves changelog quality
- Automatically categorizes commits
- Detects breaking changes
- Generates user-friendly descriptions

### 2. Choose the Right Template

✅ **Match your distribution method:**
- Open source library → Open Source template
- Web app with users → SaaS template
- REST API → API Service template
- Docker image → Container template

### 3. Enable Relevant Plugins

✅ **Only enable plugins you'll use:**
- Publishing to npm? Enable npm plugin
- Using Homebrew? Enable Homebrew plugin
- Notifying team? Enable Slack/Discord

❌ **Don't enable everything** - unused plugins slow down releases

### 4. Use Environment Variables for Secrets

✅ **ALWAYS use env vars for API keys:**
```yaml
# Good ✓
ai:
  api_key: ${OPENAI_API_KEY}

# Bad ✗ - NEVER DO THIS
ai:
  api_key: sk-proj-abc123...
```

### 5. Review Before Saving

✅ **Always review the preview:**
- Check plugin configuration
- Verify git settings
- Confirm AI provider
- Validate paths and URLs

### 6. Test After Setup

✅ **Test immediately:**
```bash
# Dry-run to verify config
relicta notes --dry-run

# Check what would happen
relicta plan
```

---

## FAQ

### Can I re-run the wizard?

Yes! Run `relicta init --guided` again. It will:
- Detect existing configuration
- Offer to backup old config
- Merge detected values with existing settings

### Can I use multiple templates?

Not directly, but you can:
1. Start with one template
2. Manually add features from other templates
3. Mix-and-match plugins

### How do I update my configuration later?

Edit `.relicta.yaml` directly:
```bash
# Edit with your preferred editor
vim .relicta.yaml
nano .relicta.yaml

# Validate changes
relicta config validate
```

### What if detection fails?

Detection is optional:
1. Skip detection (press Enter quickly)
2. Manually select project type
3. Choose appropriate template
4. Fill in questions manually

### Can I automate wizard answers?

You don't need to — for CI/CD, skip the wizard and let the zero-config default do the work:
```bash
relicta init
```

Then customize programmatically if needed:
```bash
sed -i 's/enabled: false/enabled: true/' .relicta.yaml
```

For reproducible setups, prefer committing a hand-tuned `.relicta.yaml` to your repo.

### How do I add custom plugins?

After wizard completion:
1. Edit `.relicta.yaml`
2. Add plugin to `plugins:` array
3. Validate with `relicta config validate`

Example:
```yaml
plugins:
  - name: my-custom-plugin
    enabled: true
    config:
      setting: value
```

---

## Additional Resources

### Documentation
- **Getting Started:** `docs/getting-started.md`
- **AI Providers:** `docs/ai-providers.md`
- **Configuration Reference:** `docs/configuration.md`
- **Plugin Development:** `docs/plugin-development.md`

### Examples
- **Example Configurations:** `examples/`
- **Template Files:** `internal/cli/templates/data/`

### Support
- **GitHub Issues:** https://github.com/relicta-tech/relicta/issues
- **Discussions:** https://github.com/relicta-tech/relicta/discussions
- **Documentation:** https://github.com/relicta-tech/relicta

---

## Summary

The Relicta Template Wizard:

✅ **Reduces setup time** from 30 minutes to 2 minutes
✅ **Intelligent detection** with 90%+ accuracy
✅ **10 production templates** for common project types
✅ **Beautiful terminal UI** with keyboard navigation
✅ **Smart defaults** from project analysis
✅ **Live preview** before saving
✅ **Guided AI setup** with provider selection

**Get started now:**

```bash
relicta init --guided
```

The wizard makes release automation accessible to everyone - no configuration expertise required!
