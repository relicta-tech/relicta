# ADR-008: Nox-style Plugin Distribution, Trust, and Safety Model

## Status

Accepted (2026-06-11)

## Context

Relicta's plugin runtime (ADR-002, HashiCorp go-plugin over gRPC) is solid,
but the distribution and trust layer around it lags what we ship in CI for
relicta itself, and what the sibling nox project has proven in production:

- `plugins/registry.yaml` is a single 1,400-line YAML file with one version
  per plugin and bare SHA-256 checksums. No signatures — the trust gate
  (`--allow-untrusted-plugins`) exists precisely because "signing
  infrastructure not yet shipped" (`internal/plugin/sandbox/security_notice.go`).
- Sandbox capabilities are configured by the *operator* per plugin; the
  plugin itself declares nothing, so the host cannot validate a plugin's
  actual needs against policy before loading it.
- Install pinning is manual; there is no project-manifest-driven
  auto-install, no semver constraints, no content-addressed storage.
- Several distribution CLI commands exist but are not wired into the
  command tree (`plugin registry`, `plugin search`, `plugin sandbox-status`).

Nox's plugin architecture (https://github.com/nox-hq/nox, docs/marketplace.md,
docs/plugin-authoring.md) solves these with: a static JSON registry
("no SaaS, no auth, no registry server") carrying per-artifact digests and
Sigstore Cosign keyless verification metadata; a runtime manifest in which
plugins *declare* SafetyRequirements (risk class, network hosts, file paths,
env vars) that the host validates against a trust policy before
registration; `package.json`-style required-plugin pinning with scan-time
auto-install; and a scaffold→conformance-test→tag→signed-release→registry-PR
authoring pipeline.

## Decision

Relicta adopts nox's plugin **procedure and architecture** for everything
around the runtime, while keeping the go-plugin gRPC transport (rewiring the
wire protocol would break every existing plugin for no security gain):

1. **Registry index v2** — a static JSON index (`schema_version: "2"`)
   replaces `registry.yaml` as the published format: per-plugin
   `versions[]`, per-version `capabilities`, `minimum_relicta_version`,
   and per-artifact `{os, arch, url, digest, cosign_sig_url,
   cosign_bundle_url, cosign_cert_identity_regexp, cosign_oidc_issuer}`.
   Multiple registries merge, first match wins; the official index is a
   raw GitHub URL. `sha256:tbd` placeholder digests are refused at install.

2. **Trust policies, failing closed** — three levels, configured via
   `plugins.trust_policy` or flags (`--require-signature`,
   `--allow-unverified`):
   - `permissive`: digest verification only
   - `default`: digest + Cosign keyless bundle verification (cert identity
     regexp + OIDC issuer from the index entry); unsigned artifacts fail
   - `enterprise`: digest + signature from an operator-managed keyring
   A successful Cosign verification satisfies the existing trust gate:
   verified plugins load on best-effort-sandbox platforms (macOS) without
   `--allow-untrusted-plugins`. This closes the documented signing gap.

3. **Declared SafetyRequirements** — the plugin SDK manifest grows
   `SafetyRequirements{risk_class, network_hosts, network_cidrs,
   file_paths, env_vars, needs_confirmation}` with risk classes
   `passive | active | runtime`. The host validates the declared
   requirements against the active policy *before* registering the plugin
   and derives the sandbox configuration from the declaration instead of
   requiring operators to hand-write per-plugin sandbox config. Hooks map
   to a floor risk class (e.g. notification-only = passive; `post-publish`
   tag/release mutation = active).

4. **Project-manifest pinning + auto-install** — `.relicta.yaml` gains:

   ```yaml
   plugins:
     required:
       - relicta/create-tag@^1.0
       - relicta/github-release
     registries:
       - acme=https://registry.acme.internal/relicta/index.json
     auto_install: true      # default; verify-and-install missing plugins
     trust_policy: default
   ```

   `relicta plugin install` (no args) resolves and installs everything in
   `required`; release commands auto-install missing required plugins
   unless `auto_install: false`.

5. **Content-addressed artifact cache** — extracted plugin binaries live
   under `~/.relicta/cache/artifacts/extracted/<2-char>/<sha256>/<binary>`,
   keyed by artifact digest; the manifest maps name@version → digest.
   Upgrades and rollbacks become pointer swaps; tampering invalidates the
   path.

6. **Authoring parity** — `relicta plugin create` scaffolds with a track
   profile, conformance tests (valid manifest, all declared hooks
   exercisable, deterministic manifest, risk class consistent with hooks),
   a reusable signed-release workflow, and `relicta plugin entry
   --release X.Y.Z --stamp-digests` to generate the registry index entry
   from the repo-side `plugin.yaml`.

## Consequences

- Existing plugins keep working: transport, handshake, and hook lifecycle
  are unchanged. Plugins without a SafetyRequirements declaration are
  treated as `risk_class: runtime` + unverified — loadable only under
  `permissive` policy or the legacy `--allow-untrusted-plugins` flag,
  which creates the right migration pressure.
- `registry.yaml` remains as the source the index is generated from until
  all consumers move; the published artifact is the JSON index.
- Cosign becomes a soft dependency of `relicta plugin install` under the
  `default`/`enterprise` policies (same tool our own release signing uses,
  bundle format per .goreleaser.yaml).
- Phased delivery:
  - **Phase 1**: index v2 schema + generator from registry.yaml, digest +
    Cosign verification in installer, trust policies, wire orphan CLI
    commands (`registry`, `search`, `sandbox-status`).
  - **Phase 2**: SDK SafetyRequirements + host-side policy validation +
    sandbox derivation; hook→risk-class floor map.
  - **Phase 3**: `.relicta.yaml plugins.required` + auto-install +
    content-addressed cache; conformance harness; `plugin entry`;
    reusable release workflow under relicta-tech/.github.
