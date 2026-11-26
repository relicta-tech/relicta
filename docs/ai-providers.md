# AI Providers Guide

Relicta supports multiple AI providers for generating changelogs, release notes, and other AI-powered features. This guide covers setup, configuration, and best practices for all supported providers.

## Supported Providers (2026)

| Provider | Default Model | Higher-Capability Option | Lower-Cost Option | Notes |
|----------|---------------|--------------------------|-------------------|-------|
| **OpenAI** | `gpt-6.2-sol` | `gpt-6.2-astra` | `gpt-6-luna` | Model roles follow the current OpenAI catalog |
| **Azure OpenAI** | Your deployment | Your deployment | Your deployment | Configure the deployment name from your Azure resource |
| **Anthropic Claude** | `claude-sonnet-4-6` | `claude-opus-4-8` | `claude-haiku-4-5` | Anthropic prompt caching is enabled by the adapter |
| **Google Gemini** | `gemini-3.5-flash-lite` | `gemini-3.8-flash` | `gemini-3.5-flash-lite` | Check current account and model limits |
| **Ollama** | configurable | — | — | Relicta connects to the configured Ollama endpoint |

Model IDs were checked against the provider catalogs on 2026-11-25. Recheck them before
upgrading defaults: [Anthropic model deprecations](https://docs.anthropic.com/en/docs/about-claude/model-deprecations),
[Gemini API models](https://ai.google.dev/gemini-api/docs/models) and
[deprecations](https://ai.google.dev/gemini-api/docs/deprecations),
[OpenAI API models](https://developers.openai.com/api/docs/models), and
[Azure deployment naming](https://learn.microsoft.com/en-us/azure/developer/ai/how-to/switching-endpoints).
The OpenAI entry is the general configuration default; automatic OpenAI detection selects
`gpt-6-luna` for cost-sensitive, high-volume requests.

### Per-use-case overrides

Configure `ai.model_overrides` in `.relicta.yaml` to use different models per workflow:

```yaml
ai:
  provider: anthropic
  model: claude-sonnet-4-6              # default
  model_overrides:
    risk_narrative: claude-opus-4-8     # governance-sensitive prose
    summarize_diff: claude-haiku-4-5    # fast/cheap for diff summaries
    communicate:    claude-opus-4-8     # audience-aware narratives
    release_notes:  claude-sonnet-4-6   # default is fine
```

### Anthropic Prompt Caching

Relicta's Anthropic adapter marks system prompts for ephemeral prompt caching. Whether a prompt
is cacheable and the resulting latency or billing depend on Anthropic's current model, account,
and cache rules; check the provider's documentation when estimating cost.

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
# .relicta.yaml
ai:
  enabled: true
  provider: openai
  api_key: ${OPENAI_API_KEY}
  model: gpt-6.2-sol
  max_tokens: 4096
  temperature: 0.7
```

**3. Set Environment Variable:**
```bash
export OPENAI_API_KEY="sk-proj-..."
```

**Models:**
- `gpt-6.2-sol` - Balanced model and default
- `gpt-6.2-astra` - Higher capability for complex tasks
- `gpt-6-luna` - Cost-sensitive workloads

---

### Azure OpenAI

**1. Create Azure OpenAI Resource:**
- Log in to Azure Portal
- Create "Azure OpenAI" resource
- Note: Resource endpoint and deployment name

**2. Get API Key:**
- Navigate to your Azure OpenAI resource
- Go to "Keys and Endpoint"
- Copy Key 1 or Key 2

**3. Configure:**
```yaml
# .relicta.yaml
ai:
  enabled: true
  provider: azure-openai
  api_key: ${AZURE_OPENAI_KEY}
  base_url: https://<resource-name>.openai.azure.com/openai/v1
  model: your-deployment-name
  max_tokens: 4096
  temperature: 0.7
```

**4. Set Environment Variable:**
```bash
export AZURE_OPENAI_KEY="abc123..."
export AZURE_OPENAI_ENDPOINT="https://<resource-name>.openai.azure.com"
export RELICTA_AI_MODEL="your-deployment-name"
```

With these variables and no Relicta config file, auto-detection uses the Azure `/openai/v1`
endpoint. It appends that route to a resource-root endpoint; set `RELICTA_AI_MODEL` to your
deployed model's name.

**Key Differences from OpenAI:**
- The Azure v1 endpoint is `/openai/v1`; the model field is your Azure deployment name.
- The v1 endpoint does not require a dated `api_version`.
- API key format: 32 hex characters
- If using the legacy dated endpoint, configure the resource root as `base_url` and set an
  API version supported by that resource. The adapter fallback is `2024-02-15-preview`.

**Example Base URLs:**
- Current v1: `https://myresource.openai.azure.com/openai/v1`
- Legacy dated API resource root: `https://myresource.openai.azure.com`

---

### Anthropic Claude

**1. Get API Key:**
- Sign up at https://console.anthropic.com
- Navigate to API Keys
- Create new key
- Copy key (starts with `sk-ant-...`)

**2. Configure:**
```yaml
# .relicta.yaml
ai:
  enabled: true
  provider: anthropic
  api_key: ${ANTHROPIC_API_KEY}
  model: claude-sonnet-4-6
  max_tokens: 8192
  temperature: 0.7
```

**3. Set Environment Variable:**
```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

**Models:**
- `claude-sonnet-4-6` - Balanced model and default
- `claude-opus-4-8` - Highest capability
- `claude-haiku-4-5` - Fast, cost-effective

**Notes:**
- Review the current model catalog for context limits and API availability.
- Evaluate generated notes against your own release examples before selecting a model.

---

### Google Gemini

**1. Get API Key:**
- Go to https://aistudio.google.com/app/apikey
- Sign in with Google account
- Click "Create API Key"
- Copy key (starts with `AIza...`)

**2. Configure:**
```yaml
# .relicta.yaml
ai:
  enabled: true
  provider: gemini
  api_key: ${GEMINI_API_KEY}
  model: gemini-3.5-flash-lite
  max_tokens: 8192
  temperature: 0.7
```

**3. Set Environment Variable:**
```bash
export GEMINI_API_KEY="AIza..."
```

**Models:**
- `gemini-3.8-flash` - Higher capability for complex tasks
- `gemini-3.5-flash-lite` - Fast, cost-effective default

**Notes:**
- Check current model limits, pricing, and API availability in Google's model catalog.
- Evaluate generated notes against your own release examples before selecting a model.

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
# .relicta.yaml
ai:
  enabled: true
  provider: ollama
  base_url: http://localhost:11434
  model: llama3.2  # or mistral, codellama
  max_tokens: 4096
  temperature: 0.7
```

**No API key required** - Ollama runs locally

**Example model IDs:** `llama3.2`, `mistral`, `codellama`, and `phi`. Confirm the model is
available on your Ollama server before configuring Relicta.

Ollama can run without a hosted model API key. Cost, network exposure, privacy, and hardware
requirements depend on where the Ollama server runs and which model you install.

---

## Configuration Reference

### Common Options

All providers support these configuration options:

```yaml
ai:
  enabled: true              # Enable AI features
  provider: openai           # Provider: openai, azure-openai, anthropic, gemini, ollama
  api_key: ${API_KEY}       # API key from env var (not needed for Ollama)
  model: gpt-6.2-sol        # Model name/ID
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
  base_url: https://resource.openai.azure.com/openai/v1  # Azure v1
  model: your-deployment-name
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

## Choosing a Provider

Start with the default model for the provider you already operate, then compare providers
using representative repositories and release-note drafts. Quality, latency, context limits,
availability, and price vary by model, account, region, and endpoint; they are not fixed
provider-wide properties. Check each provider's current catalog and pricing before budgeting.

| Need | Configuration to consider |
|------|---------------------------|
| Balanced OpenAI output | `gpt-6.2-sol` |
| More capable OpenAI output | `gpt-6.2-astra` |
| Lower-cost OpenAI requests | `gpt-6-luna` |
| Anthropic | Start with `claude-sonnet-4-6`; compare Opus or Haiku for your workload |
| Google Gemini | Start with `gemini-3.5-flash-lite`; compare `gemini-3.8-flash` for complex tasks |
| Azure OpenAI | Use the deployment name configured in your Azure resource |
| Local inference | Use Ollama with a model installed on your host |

For local inference, model execution stays on the configured Ollama host; Relicta still sends
requests to that host. A remote Ollama host therefore has its own network and data-handling
considerations. For hosted providers, review the provider agreement, region, retention settings,
and organizational requirements before sending repository content. Relicta's provider choice
does not itself establish a compliance certification, data residency guarantee, or privacy
commitment.

To compare quality and cost, use the same representative commit range, audience, and output
requirements for each provider. Review factual accuracy, omitted changes, tone, latency, and
token usage before choosing a default for your team.

---

## Advanced Configuration

### Multi-Provider Setup

You can configure different providers for different release types:

```yaml
# production.config.yaml - Use premium for production
ai:
  enabled: true
  provider: openai
  model: gpt-6.2-sol
  temperature: 0.7

# staging.config.yaml - Use cost-effective for staging
ai:
  enabled: true
  provider: gemini
  model: gemini-3.5-flash-lite
  temperature: 0.7
```

### Custom Prompts

Customize AI behavior with tone settings:

```yaml
ai:
  enabled: true
  provider: openai
  model: gpt-6.2-sol
  tone: technical        # Options: professional, casual, technical
  temperature: 0.5       # Lower = more focused, higher = more creative
```

### Timeout and Retry

Configure resilience settings:

```yaml
ai:
  enabled: true
  provider: openai
  model: gpt-6.2-sol
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
3. Check your provider account's current quota and configured model limits
4. Implement backoff in CI/CD pipeline

#### Context Length Exceeded

**Error:** `context length exceeded` or `maximum context length is X tokens`

**Solutions:**
1. Check the selected model's current context limit in its provider catalog.

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
   https://<resource>.openai.azure.com/openai/v1
   ```

2. Verify `model` matches the deployment name in Azure

3. For a legacy dated endpoint, set the resource root in `base_url` and configure an API version
   supported by the resource

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
1. Compare lower-cost options with a representative release:
   - OpenAI: `gpt-6-luna`
   - Gemini: `gemini-3.5-flash-lite`
   - Anthropic: `claude-haiku-4-5`

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
# Generate a preview without saving it (the configured provider is still called)
relicta notes --dry-run

# Check repository and configuration health
relicta health

# Exercise the configured provider with a draft release note
relicta notes --dry-run

# View AI-generated content
relicta notes --verbose
```

---

## Security Best Practices

### API Key Management

**DO:**
- ✅ Store API keys in environment variables
- ✅ Use separate keys for dev/staging/prod
- ✅ Rotate keys according to your organization's credential policy
- ✅ Use CI/CD secret management (GitHub Secrets, etc.)
- ✅ Apply the provider's available key and account controls

**DON'T:**
- ❌ Commit API keys to version control
- ❌ Share keys between projects
- ❌ Use production keys in development
- ❌ Log API keys in error messages
- ❌ Include keys in error reports

The wizard does not ask you to paste API keys. Set keys via environment variables and reference
them in config. A literal key placed in `.relicta.yaml` is stored as plaintext; keep secrets out
of the file.

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

### Data handling and network security

- Review the hosted provider's data handling terms and configure an endpoint and region approved
  by your organization.
- Treat repository content sent for generation as data disclosed to that provider. Commit
  messages and diffs can contain names, issue details, or other sensitive information.
- Keep API keys in environment variables or a secrets manager and avoid printing them in logs.
- Relicta sends Ollama requests to the configured endpoint. For a remote server, use network
  controls appropriate to your environment; local inference alone does not establish compliance
  or guarantee that no other process can access the data.

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
     model: gpt-6.2-sol

   # After
   ai:
     provider: gemini
     api_key: ${GEMINI_API_KEY}
     model: gemini-3.5-flash-lite
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

There is no guaranteed one-to-one model equivalent across providers. Compare candidates on
representative release data and review factual accuracy, omitted changes, tone, latency, and
token usage before switching a production workflow.

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
     model: gemini-3.5-flash-lite
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


---

## FAQ

### Can I use multiple providers?

Not simultaneously, but you can switch providers per release by changing the config or using
different config files:

```bash
relicta publish -c production.config.yaml
relicta publish -c development.config.yaml
```

### How do I compare cost and privacy?

Check current provider pricing and your organization's data-handling requirements. Cost depends
on the selected model, input and output volume, caching, account tier, and region. With Ollama,
Relicta sends requests to the configured server; verify where that server runs and who can access
it.

### Can I use custom models?

- **Azure OpenAI:** use the deployment name configured in Azure.
- **Ollama:** use a model installed and supported by the configured Ollama server.
- **Other hosted providers:** use a model ID supported by the provider API and Relicta adapter.

### How do I improve output quality?

1. Compare models using representative release changes.
2. Adjust temperature where the provider and model support it.
3. Set an appropriate tone and audience.
4. Provide clear commit messages and review the generated output before publishing.

### What happens if AI is disabled?

Relicta falls back to template-based changelog generation without AI enhancement. Basic
functionality continues to work.

### Can I run Ollama on a server?

Yes. Configure `base_url` to the server address and protect that endpoint with controls suitable
for your network and data.

### Which provider has the longest context?

Context limits vary by model and can change. Check the provider's current catalog for the exact
model you plan to use, then keep the release input within that limit.

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

Relicta integrates with OpenAI, Azure OpenAI, Anthropic, Google Gemini, and Ollama. Choose a
provider and model based on your organization's endpoint, data-handling, availability, and cost
requirements. Review generated notes before publishing them to customers.
