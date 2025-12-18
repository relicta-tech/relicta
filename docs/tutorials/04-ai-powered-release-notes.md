# Video Tutorial 4: AI-Powered Release Notes

**Duration:** 5-7 minutes
**Demo Repo:** https://github.com/relicta-tech/relicta-demo

## Overview

Use AI to generate intelligent, user-friendly release notes from your conventional commits. Supports OpenAI, Anthropic, and Ollama for local inference.

## Prerequisites

- Relicta installed
- API key for OpenAI, Anthropic, or Ollama running locally

## Script

### Scene 1: Introduction (30s)

```
"Technical commit messages aren't always user-friendly.
'fix: resolve race condition in worker pool' doesn't tell
users much about what changed for them.

Relicta's AI integration transforms your commits into
clear, professional release notes."
```

### Scene 2: Why AI-Generated Notes? (45s)

**Show comparison:**

Before (raw commits):
```
- fix: resolve race condition in worker pool
- feat: add batch processing support
- refactor: extract connection logic
- docs: update API reference
```

After (AI-enhanced):
```
## What's New in v1.3.0

### Improved Reliability
We've fixed a timing issue that could cause occasional
failures in high-concurrency scenarios.

### Batch Processing
You can now process multiple items at once, significantly
improving throughput for bulk operations.

### Documentation
Updated API reference with new endpoints and examples.
```

### Scene 3: Configuring AI Provider (1.5 min)

```yaml
# relicta.config.yaml
ai:
  enabled: true

  # Provider options: openai, anthropic, ollama
  provider: openai

  # Model selection
  model: gpt-4o-mini    # Cost-effective option
  # model: gpt-4o       # Higher quality
  # model: claude-3-5-sonnet  # For Anthropic

  # Temperature (0.0-1.0)
  # Lower = more consistent, Higher = more creative
  temperature: 0.7

  # Optional: Custom system prompt
  # system_prompt: "Generate user-friendly release notes..."
```

### Scene 4: Setting Up Authentication (1 min)

```bash
# OpenAI
export OPENAI_API_KEY="sk-xxxxxxxxxxxxxxxx"

# Anthropic
export ANTHROPIC_API_KEY="sk-ant-xxxxxxx"

# Ollama (local, no API key needed)
# Just ensure Ollama is running: ollama serve
```

**Narration:**
```
"Each provider has its own authentication method.
For local inference with Ollama, no API key is needed -
just make sure the Ollama server is running."
```

### Scene 5: Using Ollama for Local AI (1 min)

```yaml
ai:
  enabled: true
  provider: ollama
  model: llama3.2       # Or any model you have
  # endpoint: "http://localhost:11434"  # Default
```

```bash
# Pull a model
ollama pull llama3.2

# Start Ollama server
ollama serve

# Now relicta will use local inference
relicta notes
```

**Narration:**
```
"Ollama is perfect for:
- Air-gapped environments
- Cost-conscious teams
- Privacy-sensitive projects
- Experimenting with different models"
```

### Scene 6: Generating AI Notes (1 min)

```bash
# Generate release notes with AI enhancement
relicta notes
```

**Show the process:**
```
Analyzing 8 commits since v1.2.0...
Generating AI-enhanced release notes...
  ✓ Connected to OpenAI (gpt-4o-mini)
  ✓ Summarizing changes...
  ✓ Categorizing by impact...
  ✓ Writing user-friendly descriptions...

Release notes generated successfully!
```

**Show the output in CHANGELOG.md**

### Scene 7: Customizing the Output (1 min)

```yaml
ai:
  enabled: true
  provider: openai
  model: gpt-4o-mini
  temperature: 0.7

  # Custom system prompt for your project's voice
  system_prompt: |
    You are a technical writer for a developer tools company.
    Write release notes that are:
    - Clear and concise
    - Focused on user impact
    - Professional but friendly
    - Include migration notes for breaking changes
```

**Narration:**
```
"You can customize the AI's output style with a system prompt.
This is great for maintaining consistent voice across releases
or following specific documentation standards."
```

### Scene 8: AI in the Workflow (30s)

```bash
# The notes command automatically uses AI if configured
relicta plan          # Analyze changes
relicta bump          # Calculate version
relicta notes         # ← AI generates notes here
relicta approve       # Review AI output
relicta publish       # Ship it
```

**Narration:**
```
"AI enhancement happens during the notes step.
You always have a chance to review and edit before publishing."
```

### Scene 9: Cost Considerations (30s)

```
"Typical costs for AI-generated notes:

OpenAI gpt-4o-mini:
  ~$0.001 per release (pennies)

OpenAI gpt-4o:
  ~$0.01-0.05 per release

Anthropic claude-3-5-sonnet:
  ~$0.01-0.03 per release

Ollama:
  Free (local compute only)"
```

### Scene 10: Summary (30s)

```
"AI-powered release notes help you:
- Save time on changelog writing
- Create user-friendly documentation
- Maintain consistent quality
- Focus on building, not writing

Choose your provider based on quality needs,
cost constraints, and privacy requirements.

In the next video, we'll build a custom plugin
using the Relicta Plugin SDK."
```

## Tips

1. **Review before publishing**: AI isn't perfect - always review the output
2. **Start with lower temps**: 0.3-0.5 for consistent output
3. **Use gpt-4o-mini**: Best balance of quality and cost
4. **Consider Ollama**: Great for experimentation and CI/CD

## End Card

- AI docs: docs.relicta.tech/ai
- Next video: Custom Plugin Development with SDK
