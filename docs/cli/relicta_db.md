## relicta db

Manage the database backing relicta's release history

### Synopsis

Manage the database relicta keeps its release history in.

migrate, migrate-down and status drive the PostgreSQL migrator, so they require
persistence.backend to be "postgres". The connection string supports environment
expansion, so it can be kept out of the file entirely:

  persistence:
    backend: postgres
    connection_string: "${DATABASE_URL}"
    pool_size: 10

import is not tied to postgres: it reads release runs, governance memory, CGP
protocol records, and configured identity grants from .relicta into
whichever backend persistence.backend selects, so it works for sqlite too. The
one backend it refuses is "file", where the source and the destination would be
the same store.

### Options

```
  -h, --help   help for db
```

### Options inherited from parent commands

```
      --allow-untrusted-plugins   load enabled plugins (unsigned; review 'relicta plugin sandbox-status' first)
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
      --dry-run                   simulate actions without making changes
      --json                      output results as JSON
      --log string                alias for --log-level
      --log-level string          log level (debug, info, warn, error) (default "info")
      --model string              AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-6.2-sol, anthropic/claude-sonnet-4-6, local/mistral)
      --no-color                  disable colored output
      --redact                    redact secrets and API keys from output (auto-enabled in CI mode)
  -v, --verbose                   enable verbose output
```

### SEE ALSO

* [relicta](relicta.md)	 - The governance layer for software change
* [relicta db import](relicta_db_import.md)	 - Import the .relicta history into the configured database backend
* [relicta db migrate](relicta_db_migrate.md)	 - Apply all pending database migrations
* [relicta db migrate-down](relicta_db_migrate-down.md)	 - Roll back the most recently applied migration
* [relicta db status](relicta_db_status.md)	 - Show which migrations have been applied

