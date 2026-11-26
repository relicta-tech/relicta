# Custom changelog templates

Set `changelog.format` to `custom` and point `changelog.template` at a Go
`text/template` file. The template renders the sections for one release entry. Relicta writes the
version heading separately, so a template should render only the body.

```yaml
changelog:
  format: custom
  template: .relicta/changelog-entry.tmpl
```

Example `.relicta/changelog-entry.tmpl`:

```gotemplate
{{- range .Sections }}
### {{ .Title }}
{{ range .Items }}
- {{ if .Scope }}**{{ .Scope }}:** {{ end }}{{ .Description }}{{ if .CommitHash }} ({{ .CommitHash }}){{ end }}
{{- end }}
{{ end }}
```

The template receives a `communication.ChangelogEntry` with these exported fields:

- `Version`: semantic version value.
- `Date`: release date (`time.Time`), zero when dates are disabled.
- `Sections`: ordered `ChangelogSection` values.
- `CompareURL`: repository compare URL, empty when unavailable.
- `IsUnreleased`: whether this entry is unreleased.

Each section contains `Title` and `Items`. Each item contains `Description`, `Scope`,
`CommitHash`, `Author`, and `IssueRefs`. The `join`, `upper`, and `lower` template functions
are available. `group_by`, `categories`, and `exclude` are applied before the custom template
receives the entry. The template decides how item links are rendered; `CommitHash` and
`IssueRefs` are provided as raw values.

Custom templates take precedence over AI rendering because the configured template defines the
release-note format. A missing, invalid, or failing template stops notes generation with an
error; Relicta does not silently switch to a different layout.
