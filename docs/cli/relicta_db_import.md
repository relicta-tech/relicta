## relicta db import

Import the .relicta history into the configured database backend

### Synopsis

Import an existing file-based history into the configured backend.

Switching persistence.backend does not move the history that is already on disk —
migration is explicit (ADR-013). This command reads what is under .relicta and
writes it into the backend persistence.backend selects:

  * every release run under .relicta/releases, and the pointer to the current one
  * the governance record in .relicta/governance/memory.json: release records,
    incidents, decisions and execution authorizations
  * CGP proposals, decisions and authorizations under .relicta/cgp
  * governance analytics events under .relicta/governance/analytics
  * identity grants from the configured governance.identity_registry_path

Deployment records cannot move. Neither database backend has a deployments table,
so they stay in memory.json and the command says how many were left behind.

The JSON is left exactly as it was. It stays as an export until you remove it
yourself. Nothing under .relicta is deleted, moved or rewritten.

Re-running the import is safe: records are matched by ID and replaced from the
JSON, so a second run converges on the same history instead of duplicating it.
That also makes it the fix for an import that was interrupted.

  # Show what would move, without writing
  relicta db import --dry-run

  # Move it
  relicta db import

For postgres under migration_mode: manual, run 'relicta db migrate' first —
relicta does not create tables in a database you provisioned.

```
relicta db import [flags]
```

### Options

```
  -h, --help   help for import
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

* [relicta db](relicta_db.md)	 - Manage the database backing relicta's release history

