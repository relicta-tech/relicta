# AI Providers Guide

Relicta supports multiple AI providers for generating changelogs, release notes, and other AI-powered features. This guide covers setup, configuration, and best practices for all supported providers.

## Supported Providers

| Provider | Models | Best For | Pricing |
|----------|--------|----------|---------|
| **OpenAI** | GPT-4o, GPT-4, GPT-3.5 Turbo | General purpose, high quality | $0.01-$0.06/1K tokens |
| **Azure OpenAI** | GPT-4o, GPT-4, GPT-3.5 Turbo | Enterprise, compliance, private cloud | Enterprise agreements |
| **Anthropic Claude** | Claude 3.5 Sonnet, Claude 3 Opus | Long context, analysis | $0.015-$0.075/1K tokens |
| **Google Gemini** | Gemini 2.0 Flash, Gemini 1.5 Pro | Cost-effective, long context | $0.0001-$0.007/1K tokens |
| **Ollama** | Llama 3.2, Mistral, others | Local, privacy, offline | Free (self-hosted) |

## Quick Start

Choose your provider and follow the setup instructions:

### OpenAI

**1. Get API Key:**
- Sign up at https://platform.openai.com
- Navigate to API Keys
- Create new secret key
- Copy key (starts with `sk-proj-...` or `sk-...`)

**2. Configure:**
```yaml
# release.config.yaml
ai:
  enabled: true
  provider: openai
  api_key: ${OPENAI_API_KEY}
  model: gpt-4o  # or gpt-4, gpt-3.5-turbo
  max_tokens: 4096
  temperature: 0.7
```

**3. Set Environment Variable:**
```bash
export OPENAI_API_KEY="sk-proj-..."
```

**Models:**
- `gpt-4o` - Latest, fastest GPT-4 (recommended)
- `gpt-4` - High quality, slower
- `gpt-3.5-turbo` - Fast, cost-effective

---

### Azure OpenAI

**1. Create Azure OpenAI Resource:**
- Log in to Azure Portal
- Create "Azure OpenAI" resource
- Note: Resource name, deployment name, API version

**2. Get API Key:**
- Navigate to your Azure OpenAI resource
- Go to "Keys and Endpoint"
- Copy Key 1 or Key 2

**3. Configure:**
```yaml
# release.config.yaml
ai:
  enabled: true
  provider: azure-openai
  api_key: ${AZURE_OPENAI_KEY}
  base_url: https://<resource-name>.openai.azure.com/openai/deployments/<deployment-name>
  api_version: "2024-02-15-preview"
  model: gpt-4  # Your deployment name
  max_tokens: 4096
  temperature: 0.7
```

**4. Set Environment Variable:**
```bash
export AZURE_OPENAI_KEY="abc123..."
```

**Key Differences from OpenAI:**
- Requires `base_url` with your Azure resource and deployment
- Requires `api_version` (latest: `2024-02-15-preview`)
- API key format: 32 hex characters
- Model name matches your deployment name

**Example Base URLs:**
- Single model: `https://myresource.openai.azure.com/openai/deployments/gpt-4`
- With version: `https://myresource.openai.azure.com/openai/deployments/gpt-4/chat/completions?api-version=2024-02-15-preview`

---

### Anthropic Claude

**1. Get API Key:**
- Sign up at https://console.anthropic.com
- Navigate to API Keys
- Create new key
- Copy key (starts with `sk-ant-...`)

**2. Configure:**
```yaml
# release.config.yaml
ai:
  enabled: true
  provider: anthropic
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-3-5-sonnet-20241022  # or claude-3-opus-20240229
  max_tokens: 8192
  temperature: 0.7
```

**3. Set Environment Variable:**
```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

**Models:**
- `claude-3-5-sonnet-20241022` - Latest Sonnet (recommended)
- `claude-3-opus-20240229` - Highest quality, slowest
- `claude-3-haiku-20240307` - Fast, cost-effective

**Strengths:**
- 200K token context window (excellent for large changelogs)
- Strong at analysis and summarization
- Constitutional AI for safer outputs

---

### Google Gemini

**1. Get API Key:**
- Go to https://aistudio.google.com/app/apikey
- Sign in with Google account
- Click "Create API Key"
- Copy key (starts with `AIza...`)

**2. Configure:**
```yaml
# release.config.yaml
ai:
  enabled: true
  provider: gemini
  api_key: ${GEMINI_API_KEY}
  model: gemini-2.0-flash-exp  # or gemini-1.5-pro, gemini-1.5-flash
  max_tokens: 8192
  temperature: 0.7
```

**3. Set Environment Variable:**
```bash
export GEMINI_API_KEY="AIza..."
```

**Models:**
- `gemini-2.0-flash-exp` - Latest experimental (recommended, fastest)
- `gemini-1.5-pro` - High quality, large context
- `gemini-1.5-flash` - Fast, cost-effective

**Strengths:**
- Very cost-effective ($0.0001-$0.007/1K tokens)
- Up to 2M token context window
- Multimodal capabilities (future use)

---

### Ollama (Local)

**1. Install Ollama:**
```bash
# macOS/Linux
curl -fsSL https://ollama.com/install.sh | sh

# Windows
# Download from https://ollama.com/download
```

**2. Pull Model:**
```bash
ollama pull llama3.2  # or mistral, codellama, etc.
ollama list           # View installed models
```

**3. Start Ollama Server:**
```bash
ollama serve
# Server runs at http://localhost:11434
```

**4. Configure:**
```yaml
# release.config.yaml
ai:
  enabled: true
  provider: ollama
  base_url: http://localhost:11434
  model: llama3.2  # or mistral, codellama
  max_tokens: 4096
  temperature: 0.7
```

**No API key required** - Ollama runs locally

**Popular Models:**
- `llama3.2` - Meta's latest (recommended)
- `mistral` - Fast, high quality
- `codellama` - Code-focused
- `phi` - Microsoft's small model

**Strengths:**
- 100% private - data never leaves your machine
- No API costs
- Works offline
- Full control over model selection

**Requirements:**
- 8GB+ RAM (16GB recommended)
- 4-8GB disk space per model
- CPU or GPU (GPU much faster)

---

## Configuration Reference

### Common Options

All providers support these configuration options:

```yaml
ai:
  enabled: true              # Enable AI features
  provider: openai           # Provider: openai, azure-openai, anthropic, gemini, ollama
  api_key: ${API_KEY}       # API key from env var (not needed for Ollama)
  model: gpt-4o             # Model name/ID
  max_tokens: 4096          # Maximum response length
  temperature: 0.7          # Creativity (0.0-1.0)
  tone: professional        # Tone: professional, casual, technical
```

### Provider-Specific Options

#### OpenAI / Azure OpenAI
```yaml
ai:
  provider: openai
  base_url: https://api.openai.com/v1  # Custom endpoint (optional)
  organization: org-...                 # Organization ID (optional)
```

#### Azure OpenAI
```yaml
ai:
  provider: azure-openai
  base_url: https://resource.openai.azure.com/openai/deployments/deployment  # Required
  api_version: "2024-02-15-preview"                                           # Required
```

#### Anthropic
```yaml
ai:
  provider: anthropic
  # No additional options
```

#### Gemini
```yaml
ai:
  provider: gemini
  # No additional options
```

#### Ollama
```yaml
ai:
  provider: ollama
  base_url: http://localhost:11434  # Ollama server URL
  # No API key needed
```

---

## Provider Comparison

### Quality & Performance

| Provider | Quality | Speed | Context Length | Best Use Case |
|----------|---------|-------|----------------|---------------|
| **OpenAI GPT-4o** | ★★★★★ | ★★★★★ | 128K tokens | General purpose, balanced |
| **OpenAI GPT-4** | ★★★★★ | ★★★☆☆ | 128K tokens | Highest quality |
| **Azure OpenAI** | ★★★★★ | ★★★★☆ | 128K tokens | Enterprise compliance |
| **Claude 3.5 Sonnet** | ★★★★★ | ★★★★☆ | 200K tokens | Large projects, analysis |
| **Claude 3 Opus** | ★★★★★ | ★★★☆☆ | 200K tokens | Complex analysis |
| **Gemini 2.0 Flash** | ★★★★☆ | ★★★★★ | 2M tokens | Cost-effective, fast |
| **Gemini 1.5 Pro** | ★★★★☆ | ★★★★☆ | 2M tokens | Large context |
| **Ollama (Llama 3.2)** | ★★★★☆ | ★★★☆☆ | 128K tokens | Privacy, offline |

### Cost Comparison

**Input Pricing** (per 1M tokens):

| Provider | Entry Model | Mid Model | Premium Model |
|----------|-------------|-----------|---------------|
| **OpenAI** | $0.50 (3.5-turbo) | $10 (GPT-4o) | $30 (GPT-4) |
| **Azure OpenAI** | Enterprise pricing | Enterprise pricing | Enterprise pricing |
| **Anthropic** | $3 (Haiku) | $15 (Sonnet) | $75 (Opus) |
| **Gemini** | $0.075 (Flash) | $1.25 (Pro) | $7 (Pro 1.5) |
| **Ollama** | Free | Free | Free |

**Output Pricing** is typically 2-3x input pricing.

**Estimated Monthly Cost** (1000 releases/month, avg 2K tokens):

| Provider | Estimated Cost |
|----------|---------------|
| **OpenAI GPT-4o** | ~$20-40 |
| **Anthropic Sonnet** | ~$30-60 |
| **Gemini Flash** | ~$0.15-0.30 |
| **Ollama** | $0 (hardware costs only) |

### Feature Support

| Feature | OpenAI | Azure | Claude | Gemini | Ollama |
|---------|--------|-------|--------|--------|--------|
| Changelog Generation | ✅ | ✅ | ✅ | ✅ | ✅ |
| Release Notes | ✅ | ✅ | ✅ | ✅ | ✅ |
| Commit Categorization | ✅ | ✅ | ✅ | ✅ | ✅ |
| Breaking Change Detection | ✅ | ✅ | ✅ | ✅ | ✅ |
| Custom Prompts | ✅ | ✅ | ✅ | ✅ | ✅ |
| Streaming Responses | ✅ | ✅ | ✅ | ❌ | ✅ |
| Function Calling | ✅ | ✅ | ✅ | ✅ | ❌ |
| Long Context (>100K) | ✅ | ✅ | ✅ | ✅ | ✅* |

*Depends on model

---

## Choosing a Provider

### Decision Matrix

**Choose OpenAI if you want:**
- ✅ Best overall quality-to-speed ratio
- ✅ Most popular, well-documented
- ✅ Latest features and capabilities
- ✅ Reliable, stable API

**Choose Azure OpenAI if you need:**
- ✅ Enterprise compliance (GDPR, HIPAA, SOC 2)
- ✅ Private deployment in your Azure tenant
- ✅ Data residency requirements
- ✅ Microsoft enterprise support

**Choose Anthropic Claude if you need:**
- ✅ Very long context windows (200K tokens)
- ✅ Strong analysis and summarization
- ✅ Constitutional AI safety
- ✅ Alternative to OpenAI

**Choose Google Gemini if you want:**
- ✅ Most cost-effective option
- ✅ Massive context windows (2M tokens)
- ✅ Fast inference speeds
- ✅ Future multimodal capabilities

**Choose Ollama if you need:**
- ✅ 100% privacy - no data leaves your machine
- ✅ Offline operation
- ✅ No API costs
- ✅ Full control over infrastructure

### Recommendations by Use Case

| Use Case | Recommended Provider | Why |
|----------|---------------------|-----|
| **Startup/Individual** | Gemini Flash | Best cost-performance ratio |
| **Small Team** | OpenAI GPT-4o | Balanced quality and speed |
| **Enterprise** | Azure OpenAI | Compliance, security, support |
| **Large Projects** | Claude 3.5 Sonnet | Large context, great analysis |
| **Privacy-Critical** | Ollama | Local, no data sharing |
| **High Volume** | Gemini 1.5 Pro | Cost-effective at scale |
| **Offline/Air-Gapped** | Ollama | Works without internet |

---

## Advanced Configuration

### Multi-Provider Setup

You can configure different providers for different release types:

```yaml
# production.config.yaml - Use premium for production
ai:
  enabled: true
  provider: openai
  model: gpt-4o
  temperature: 0.7

# staging.config.yaml - Use cost-effective for staging
ai:
  enabled: true
  provider: gemini
  model: gemini-2.0-flash-exp
  temperature: 0.7
```

### Custom Prompts

Customize AI behavior with tone settings:

```yaml
ai:
  enabled: true
  provider: openai
  model: gpt-4o
  tone: technical        # Options: professional, casual, technical
  temperature: 0.5       # Lower = more focused, higher = more creative
```

### Timeout and Retry

Configure resilience settings:

```yaml
ai:
  enabled: true
  provider: openai
  model: gpt-4o
  timeout: 30s           # Request timeout (default: 30s)
  max_retries: 3         # Retry count (default: 3)
```

---

## Troubleshooting

### Common Issues

#### API Key Invalid

**Error:** `authentication failed` or `invalid API key`

**Solutions:**
1. Verify API key format:
   - OpenAI: `sk-proj-...` or `sk-...`
   - Azure: 32 hex characters
   - Anthropic: `sk-ant-...`
   - Gemini: `AIza...`

2. Check environment variable:
   ```bash
   echo $OPENAI_API_KEY  # Should print your key
   ```

3. Regenerate API key from provider dashboard

#### Rate Limit Exceeded

**Error:** `rate limit exceeded` or `429 Too Many Requests`

**Solutions:**
1. Reduce frequency of releases
2. Upgrade to higher tier plan
3. Switch to provider with higher limits (Gemini, Ollama)
4. Implement backoff in CI/CD pipeline

#### Context Length Exceeded

**Error:** `context length exceeded` or `maximum context length is X tokens`

**Solutions:**
1. Switch to model with larger context:
   - OpenAI: GPT-4 (128K)
   - Claude: Sonnet/Opus (200K)
   - Gemini: Pro (2M)

2. Reduce changelog size by filtering commits:
   ```yaml
   changelog:
     exclude_types:
       - chore
       - docs
       - style
   ```

3. Decrease `max_tokens` setting

#### Azure OpenAI Issues

**Error:** `Resource not found` or `Deployment not found`

**Solutions:**
1. Verify `base_url` format:
   ```
   https://<resource>.openai.azure.com/openai/deployments/<deployment>
   ```

2. Check `api_version` is current (use `2024-02-15-preview`)

3. Ensure deployment name matches `model` field

4. Verify resource region supports the model

#### Ollama Connection Failed

**Error:** `connection refused` or `dial tcp: connect: connection refused`

**Solutions:**
1. Ensure Ollama server is running:
   ```bash
   ollama serve
   ```

2. Verify base URL:
   ```yaml
   ai:
     base_url: http://localhost:11434  # Default
   ```

3. Check firewall rules

4. Verify model is downloaded:
   ```bash
   ollama list
   ollama pull llama3.2
   ```

#### Slow Response Times

**Symptoms:** Releases taking >30 seconds

**Solutions:**
1. Switch to faster model:
   - OpenAI: GPT-4o (not GPT-4)
   - Gemini: Flash (not Pro)
   - Claude: Haiku (not Opus)

2. Reduce `max_tokens`:
   ```yaml
   ai:
     max_tokens: 2048  # Instead of 4096
   ```

3. Use local Ollama with GPU acceleration

4. Increase timeout:
   ```yaml
   ai:
     timeout: 60s
   ```

### Testing Configuration

Verify your AI setup:

```bash
# Test with dry-run
relicta notes --dry-run

# Check AI provider connectivity
relicta health  # (if implemented)

# View AI-generated content
relicta notes --verbose
```

---

## Security Best Practices

### API Key Management

**DO:**
- ✅ Store API keys in environment variables
- ✅ Use separate keys for dev/staging/prod
- ✅ Rotate keys regularly (every 90 days)
- ✅ Use CI/CD secret management (GitHub Secrets, etc.)
- ✅ Restrict key permissions (read-only where possible)

**DON'T:**
- ❌ Commit API keys to version control
- ❌ Share keys between projects
- ❌ Use production keys in development
- ❌ Log API keys in error messages
- ❌ Include keys in error reports

### Environment Variable Examples

```bash
# ~/.bashrc or ~/.zshrc
export OPENAI_API_KEY="sk-proj-..."
export ANTHROPIC_API_KEY="sk-ant-..."
export GEMINI_API_KEY="AIza..."
export AZURE_OPENAI_KEY="abc123..."

# CI/CD (GitHub Actions)
# Set in repository Settings > Secrets
${{ secrets.OPENAI_API_KEY }}
```

### Network Security

**For cloud providers (OpenAI, Claude, Gemini):**
- All communication uses TLS 1.2+
- API keys sent via HTTP headers, not URLs
- No PII or sensitive data in prompts

**For Ollama (local):**
- No external network access required
- Data stays on your machine
- Consider firewall rules if exposing to network

### Compliance Considerations

| Provider | GDPR | HIPAA | SOC 2 | Data Residency |
|----------|------|-------|-------|----------------|
| **OpenAI** | ✅ | ❌ | ✅ | US |
| **Azure OpenAI** | ✅ | ✅ | ✅ | Configurable |
| **Anthropic** | ✅ | ❌ | ✅ | US |
| **Gemini** | ✅ | ❌ | ✅ | US/EU |
| **Ollama** | ✅ | ✅ | N/A | Local |

---

## Migration Guide

### Switching Providers

To switch from one provider to another:

1. **Get new API key** from target provider

2. **Update configuration:**
   ```yaml
   # Before
   ai:
     provider: openai
     api_key: ${OPENAI_API_KEY}
     model: gpt-4o

   # After
   ai:
     provider: gemini
     api_key: ${GEMINI_API_KEY}
     model: gemini-2.0-flash-exp
   ```

3. **Set new environment variable:**
   ```bash
   export GEMINI_API_KEY="AIza..."
   ```

4. **Test with dry-run:**
   ```bash
   relicta notes --dry-run
   ```

5. **Verify output quality** matches expectations

### Model Equivalents

When migrating, use these equivalent models:

| OpenAI | Azure | Claude | Gemini | Ollama |
|--------|-------|--------|--------|--------|
| GPT-4o | GPT-4o | Sonnet 3.5 | Flash 2.0 | Llama 3.2 |
| GPT-4 | GPT-4 | Opus 3 | Pro 1.5 | Mistral |
| GPT-3.5 | GPT-3.5 | Haiku 3 | Flash 1.5 | Phi |

---

## Performance Optimization

### Token Usage Optimization

Reduce costs by optimizing token usage:

1. **Exclude unnecessary commits:**
   ```yaml
   changelog:
     exclude_types:
       - chore
       - docs
       - style
       - test
   ```

2. **Limit commit history:**
   ```yaml
   changelog:
     max_commits: 100  # Only analyze last 100 commits
   ```

3. **Use smaller models for simple releases:**
   ```yaml
   ai:
     model: gemini-1.5-flash  # Instead of gemini-1.5-pro
   ```

### Caching Strategies

Relicta caches AI responses by default:

```yaml
ai:
  enabled: true
  cache_enabled: true    # Cache AI responses (default: true)
  cache_ttl: 24h        # Cache lifetime (default: 24h)
```

Cached responses are stored in `.relicta/cache/` (gitignored).

### Batch Processing

For large projects with many releases, consider:

1. **Batch releases weekly** instead of per-commit
2. **Use Gemini for cost savings** at scale
3. **Enable changelog caching** to reuse AI outputs
4. **Optimize prompts** to reduce token usage

---

## FAQ

### Can I use multiple providers?

Not simultaneously, but you can switch providers per release by changing the config or using different config files:

```bash
# Use OpenAI for production
relicta publish -c production.config.yaml

# Use Gemini for development
relicta publish -c development.config.yaml
```

### Which provider is most cost-effective?

**Gemini Flash** is the most cost-effective at $0.075/1M input tokens - 10-100x cheaper than alternatives while maintaining good quality.

### Which provider is most private?

**Ollama** is the only provider that keeps data 100% local. No API calls, no data sharing, fully offline capable.

### Can I use custom models?

- **OpenAI**: Only official models supported
- **Azure**: Any Azure-deployed model
- **Claude**: Only official models
- **Gemini**: Only official models
- **Ollama**: Any Ollama-compatible model (dozens available)

### How do I improve output quality?

1. Use higher-quality models (GPT-4, Opus, Pro)
2. Adjust temperature (lower = more focused)
3. Set appropriate tone (professional, technical, casual)
4. Provide good commit messages (garbage in, garbage out)
5. Use conventional commits for better categorization

### What happens if AI is disabled?

Relicta falls back to template-based changelog generation without AI enhancement. Basic functionality continues to work.

### Can I run Ollama on a server?

Yes! Ollama can run on any Linux/macOS server:

```bash
# On server
ollama serve --host 0.0.0.0:11434

# In config
ai:
  provider: ollama
  base_url: http://your-server:11434
```

### Which provider has the longest context?

**Gemini** has the longest context at 2M tokens, followed by **Claude** at 200K tokens.

---

## Additional Resources

### Official Documentation

- **OpenAI:** https://platform.openai.com/docs
- **Azure OpenAI:** https://learn.microsoft.com/azure/ai-services/openai/
- **Anthropic Claude:** https://docs.anthropic.com
- **Google Gemini:** https://ai.google.dev/docs
- **Ollama:** https://ollama.com/docs

### API References

- **OpenAI API:** https://platform.openai.com/docs/api-reference
- **Azure OpenAI API:** https://learn.microsoft.com/azure/ai-services/openai/reference
- **Claude API:** https://docs.anthropic.com/claude/reference
- **Gemini API:** https://ai.google.dev/api
- **Ollama API:** https://github.com/ollama/ollama/blob/main/docs/api.md

### Pricing Calculators

- **OpenAI Pricing:** https://openai.com/pricing
- **Azure Pricing:** https://azure.microsoft.com/pricing/calculator/
- **Claude Pricing:** https://www.anthropic.com/pricing
- **Gemini Pricing:** https://ai.google.dev/pricing

### Support

- **Relicta Issues:** https://github.com/relicta-tech/relicta/issues
- **Documentation:** https://github.com/relicta-tech/relicta
- **Discussions:** https://github.com/relicta-tech/relicta/discussions

---

## Summary

Relicta supports 5 AI providers, each with unique strengths:

- **OpenAI** - Best overall, industry standard
- **Azure OpenAI** - Enterprise compliance and security
- **Anthropic Claude** - Long context, strong analysis
- **Google Gemini** - Most cost-effective, massive context
- **Ollama** - 100% private, free, offline

Choose based on your priorities: quality, cost, privacy, or compliance. All providers deliver excellent results for changelog and release note generation.

For most users, we recommend starting with **Gemini Flash** for cost-effectiveness or **OpenAI GPT-4o** for balanced performance.
