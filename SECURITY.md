# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 2.x.x   | :white_check_mark: |
| 1.x.x   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

1. **Do not** open a public GitHub issue for security vulnerabilities
2. Email security concerns to: security@relicta.tech (or create a private security advisory on GitHub)
3. Include as much detail as possible:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

### What to Expect

- **Acknowledgment**: We will acknowledge receipt within 48 hours
- **Assessment**: We will assess the vulnerability and determine its severity
- **Updates**: We will keep you informed of our progress
- **Resolution**: We aim to resolve critical vulnerabilities within 7 days
- **Credit**: We will credit reporters in the release notes (unless anonymity is requested)

## Security Measures

### Code Security

- **Static Analysis**: All code is scanned with [gosec](https://github.com/securego/gosec) for security vulnerabilities
- **CodeQL**: GitHub CodeQL analysis runs on every push and PR
- **Dependency Scanning**: Dependabot monitors and updates vulnerable dependencies
- **SARIF Integration**: Security findings are uploaded to GitHub Security tab

### Supply Chain Security

- **Signed Releases**: All release artifacts are signed
- **Checksum Verification**: SHA256 checksums provided for all binaries
- **Minimal Dependencies**: We minimize external dependencies to reduce attack surface
- **Dependency Review**: All dependency changes are reviewed before merging

### Runtime Security

- **No Network by Default**: Relicta operates offline unless explicitly configured with plugins
- **Least Privilege**: Plugins operate with minimal required permissions
- **Input Validation**: All user input is validated and sanitized
- **Secure Defaults**: Security-focused default configurations

### CI/CD Security

- **Least Privilege Permissions**: All GitHub Actions workflows use minimal required permissions
- **Pin Action Versions**: Dependencies are pinned to specific versions
- **Secret Scanning**: GitHub secret scanning enabled
- **Branch Protection**: Main branch requires reviews and passing checks

## Security Best Practices for Users

### Configuration

```yaml
# .relicta.yaml - Security recommendations
ai:
  # Use environment variables for API keys, never hardcode
  provider: openai  # API key via OPENAI_API_KEY env var

plugins:
  - name: github
    enabled: true
    config:
      # Use GITHUB_TOKEN from CI/CD, avoid personal tokens in config
```

### Environment Variables

- Store sensitive values in environment variables
- Use CI/CD secret management (GitHub Secrets, Vault, etc.)
- Never commit secrets to version control
- Rotate API keys periodically

### Plugin Security

- Only enable plugins you need
- Review plugin permissions before enabling
- Use official plugins from trusted sources
- Monitor plugin network activity if concerned

## Vulnerability Disclosure Timeline

| Severity | Target Resolution |
| -------- | ----------------- |
| Critical | 7 days            |
| High     | 14 days           |
| Medium   | 30 days           |
| Low      | 90 days           |

## Security Updates

Security updates are released as patch versions (e.g., 2.0.1) and announced via:
- GitHub Releases
- GitHub Security Advisories
- Release notes in CHANGELOG.md

We recommend enabling GitHub watch notifications for this repository to stay informed of security updates.
