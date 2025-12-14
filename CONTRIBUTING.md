# Contributing to Relicta

Thank you for your interest in contributing to Relicta! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone.

## Getting Started

### Prerequisites

- Go 1.22 or later
- Git
- Make (optional, for using Makefile commands)

### Setting Up the Development Environment

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/relicta.git
   cd relicta
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/relicta-tech/relicta.git
   ```
4. Install dependencies:
   ```bash
   go mod download
   ```

### Building

```bash
# Using Make
make build

# Or directly with Go
go build -o relicta ./cmd/relicta
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run tests for a specific package
go test ./internal/service/git/...
```

### Linting

```bash
# Using Make
make lint

# Install golangci-lint if not available
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feature/add-npm-plugin` - New features
- `fix/version-calculation` - Bug fixes
- `docs/update-readme` - Documentation changes
- `refactor/git-service` - Code refactoring

### Commit Messages

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

#### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation changes |
| `style` | Code style changes (formatting, semicolons, etc.) |
| `refactor` | Code refactoring |
| `perf` | Performance improvements |
| `test` | Adding or updating tests |
| `build` | Build system or external dependencies |
| `ci` | CI/CD configuration |
| `chore` | Other changes that don't modify src or test files |

#### Examples

```
feat(ai): add Anthropic Claude support

Add support for Anthropic's Claude models as an alternative AI provider.
Includes rate limiting and retry logic.

Closes #42
```

```
fix(version): handle pre-release versions correctly

Previously, pre-release versions were not being parsed correctly,
leading to incorrect version bumps.
```

### Pull Request Process

1. Create a new branch from `main`:
   ```bash
   git checkout main
   git pull upstream main
   git checkout -b feature/your-feature
   ```

2. Make your changes, following the code style guidelines

3. Write or update tests for your changes

4. Ensure all tests pass:
   ```bash
   make test
   ```

5. Ensure code passes linting:
   ```bash
   make lint
   ```

6. Commit your changes using conventional commits

7. Push to your fork:
   ```bash
   git push origin feature/your-feature
   ```

8. Open a Pull Request on GitHub

### Pull Request Guidelines

- Provide a clear description of the changes
- Link any related issues
- Include screenshots for UI changes
- Ensure CI checks pass
- Request review from maintainers

## Code Style

### Go Guidelines

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Write descriptive variable and function names
- Add comments for exported functions and types
- Keep functions focused and small
- Handle errors explicitly

### Project Structure

```
├── cmd/relicta/     # CLI entry point
├── internal/
│   ├── application/       # Use cases (application layer)
│   ├── domain/           # Business logic (domain layer)
│   ├── infrastructure/   # External adapters
│   ├── cli/              # Command implementations
│   ├── service/          # Application services
│   ├── config/           # Configuration
│   ├── state/            # State management
│   ├── errors/           # Error types
│   └── ui/               # Terminal UI components
├── pkg/plugin/           # Public plugin interface
├── plugins/              # Official plugins
├── templates/            # Template files
└── test/                 # Integration and E2E tests
```

### Testing Guidelines

- Write table-driven tests where appropriate
- Use meaningful test names that describe the scenario
- Test both success and error cases
- Mock external dependencies
- Aim for at least 75% code coverage on new code

Example:
```go
func TestCalculateNextVersion(t *testing.T) {
    tests := []struct {
        name        string
        current     string
        releaseType ReleaseType
        want        string
    }{
        {
            name:        "major bump",
            current:     "1.0.0",
            releaseType: Major,
            want:        "2.0.0",
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := CalculateNextVersion(tt.current, tt.releaseType)
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

## Adding New Features

### Adding a New Command

1. Create a new file in `internal/cli/`
2. Define the command using Cobra
3. Add it to the root command in `root.go`
4. Write tests for the command
5. Update documentation

### Adding a New Plugin

1. Create a new directory in `plugins/`
2. Implement the `pkg/plugin.Plugin` interface
3. Add plugin documentation to README
4. Write tests for the plugin

### Adding a New AI Provider

1. Add the provider in `internal/service/ai/`
2. Implement the `Service` interface
3. Add configuration options
4. Write tests for the provider

## Reporting Issues

### Bug Reports

Include:
- Relicta version
- Go version
- Operating system
- Steps to reproduce
- Expected behavior
- Actual behavior
- Relevant logs or error messages

### Feature Requests

Include:
- Clear description of the feature
- Use case or problem it solves
- Proposed implementation (optional)
- Alternatives considered (optional)

## Security

If you discover a security vulnerability, please do NOT open a public issue. Instead, send a private message to the maintainers.

## Questions?

Feel free to open an issue for questions or join the discussions on GitHub.

## License

By contributing to Relicta, you agree that your contributions will be licensed under the MIT License.
