# Relicta Release Supply Chain Security

Relicta release archives are checksummed, the checksum manifest is signed with
Sigstore keyless signing, and GitHub build provenance attestations are generated
for the archives and checksum manifest. These controls are implemented by
`.goreleaser.yaml` and `.github/workflows/release.yaml`.

## Release artifacts

Each tagged release publishes its platform archives, `checksums.txt`, and the
Sigstore bundle `checksums.txt.sigstore.json`. Archive names follow
`relicta_<OS>_<ARCH>.tar.gz`; Windows uses `.zip`. Releases without Apple
credentials contain Linux amd64/arm64 and Windows amd64. With all Apple
credentials configured, macOS amd64/arm64 archives are included too.

The SHA-256 manifest covers the release archives. Its Sigstore bundle signs the
manifest, allowing a verifier to authenticate the checksums before using them.
GitHub artifact attestations provide build provenance for each archive and the
manifest. Neither control proves that the software is free of defects; review
the source and build workflow as part of your trust decision.

The published v4.2.0 release includes unsigned Darwin archives. New tag builds
include Darwin archives only when all Apple signing and notarization credentials
are configured; otherwise the workflow selects the Linux/Windows-only release
configuration. Check the assets for the exact release tag before choosing a
platform archive.

## Verify an archive

Install `cosign` and the GitHub CLI (`gh`), then set `VERSION` to the release
tag and `ARCHIVE` to the platform archive you need:

```bash
set -euo pipefail
VERSION=v4.2.0
ARCHIVE=relicta_linux_amd64.tar.gz
if [[ ! "$VERSION" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "VERSION must be a stable Relicta release tag (for example v4.2.0)" >&2
  exit 1
fi
identity_version="${VERSION//./\\.}"
identity="^https://github\.com/relicta-tech/relicta/\.github/workflows/release\.yaml@refs/tags/${identity_version}$"
BASE="https://github.com/relicta-tech/relicta/releases/download/${VERSION}"

curl -fL "${BASE}/checksums.txt" -o checksums.txt
curl -fL "${BASE}/checksums.txt.sigstore.json" -o checksums.txt.sigstore.json
curl -fL "${BASE}/${ARCHIVE}" -o "${ARCHIVE}"
cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp "${identity}" \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt

awk -v asset="${ARCHIVE}" '$2 == asset { print; found++ } END { if (found != 1) exit 1 }' checksums.txt | sha256sum --check
gh attestation verify "${ARCHIVE}" \
  --repo relicta-tech/relicta \
  --signer-workflow relicta-tech/relicta/.github/workflows/release.yaml \
  --source-ref "refs/tags/${VERSION}"
```

On macOS, replace the checksum command with:

```bash
set -euo pipefail
awk -v asset="${ARCHIVE}" '$2 == asset { print; found++ } END { if (found != 1) exit 1 }' checksums.txt | shasum -a 256 --check
```

For a verified Windows amd64 install, use the end-to-end [PowerShell procedure](../quick-start.md#verified-windows-binary-amd64). It authenticates the checksum manifest, checks the archive digest, and verifies provenance before extraction. Do not extract or execute an archive if any verification step fails.

To verify the checksum manifest's GitHub attestation as well:

```bash
gh attestation verify checksums.txt \
  --repo relicta-tech/relicta \
  --signer-workflow relicta-tech/relicta/.github/workflows/release.yaml \
  --source-ref "refs/tags/${VERSION}"
```

The exact version and filenames are release-specific. Confirm that the archive
appears in that release's assets before using these commands.

## Container images

After the protected release approval is granted, the workflow builds a
multi-platform image for `linux/amd64` and `linux/arm64`, including BuildKit
SBOM and provenance metadata. It first pushes
the image by digest without release tags, scans both platforms for HIGH and
CRITICAL OS-package and application-library vulnerabilities, then publishes
GitHub provenance and an SPDX SBOM attestation for that same digest. These
checks must succeed before the binary GitHub Release is published. The version,
major/minor, major, and commit-SHA image tags are added only after both the
verified image and binary release succeed. A failed check may leave an
untagged image manifest in GHCR; no release version tag points to it.

To verify provenance for an image tag, set `VERSION` to the image's semantic
version without the leading `v` (for example `4.2.0`), install the GitHub CLI,
and authenticate to GHCR if the package is private. The source ref uses the
corresponding Git tag, which includes the leading `v`:

The currently published `4.2.0` image predates these workflow controls. Its GHCR
tag exists, but GitHub has no provenance attestation for its digest (checked
2026-11-25), so the command below returns 404 for `4.2.0`. Use it only with a
tag published after the attestation workflow is merged and has completed
successfully; these controls do not apply retroactively.
The published `4.2.0` image was also scanned with Trivy on 2026-11-26 and had
60 HIGH/CRITICAL findings: 37 in Alpine packages and 23 in the Go binary (59
HIGH, 1 CRITICAL). Do not deploy that image to production. Check the exact image
digest for the fixed release before deploying; a valid signature or provenance
attestation establishes artifact identity and origin, not vulnerability status.

```bash
: "${VERSION:?Set VERSION to a semantic version published by the attestation-enabled workflow, for example 5.0.0}"
IMAGE="ghcr.io/relicta-tech/relicta:${VERSION}"
SOURCE_REF="refs/tags/v${VERSION}"
# Log in only if this package is private in your environment.
# docker login ghcr.io
gh attestation verify "oci://${IMAGE}" --repo relicta-tech/relicta \
  --signer-workflow relicta-tech/relicta/.github/workflows/release.yaml \
  --source-ref "${SOURCE_REF}"
```

The verified subject is the image digest behind the tag, so the same attestations
also apply to other release tags pointing to that digest. The image SBOM is
published as an SPDX attestation to GHCR alongside provenance. Consumers can
download the registry attestations with `gh attestation download` and inspect the
SPDX predicate; provenance verification alone does not assert that the SBOM is
complete or that the image has no vulnerabilities.

## Build and platform notes

The release workflow builds with GoReleaser on GitHub-hosted runners and uses
GitHub OIDC for keyless Sigstore signing and artifact attestations. When all
five Apple Developer ID and App Store Connect secrets are configured, tagged
releases include signed and notarized macOS binaries. When none are configured,
the release publishes Linux and Windows binaries only; partial configuration
fails closed. Manual snapshot builds remain unsigned for review and do not
publish.

### Configure the release workflow

Before enabling tag-triggered releases, configure these controls in the
`relicta-tech/relicta` repository:

1. Protect the default branch in **Settings → Rules → Rulesets** (or an
   [equivalent branch protection rule](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)):
   require pull requests, at least one
   approving review, approval of the most recent reviewable push, and resolved
   review conversations. Require the successful CI, security, and release
   validation checks emitted by the workflows for the candidate branch. See
   GitHub's [available ruleset rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets).
   Select check names from a successful run on the candidate; GitHub only offers check
   contexts it has observed, and stale or guessed names can leave the rule
   ineffective. Block force pushes and branch deletion. Confirm a pull request
   cannot merge with a failing required check or without its required review.
   If the repository has a single maintainer, arrange an independent reviewer
   before making this rule mandatory.
2. In **Settings → Environments**, create a GitHub Actions environment named
   `release-approval` (the name must match exactly). Under **Deployment
   protection rules**, add at least one required reviewer and enable
   **Prevent self-review**. Choose a reviewer who can approve repository
   deployments and is different from the person who starts the release. The
   workflow waits for that environment approval, then checks through the GitHub
   API that the environment still has a required reviewer and prevents
   self-review. It stops before image push or release publication if the
   environment or either protection is missing. Recheck the environment after
   changing repository or organization rules, and before the first tagged
   release.
3. Add the repository secret `HOMEBREW_TAP_TOKEN` for stable-version formula
   updates. Give its fine-grained token access only to `relicta-tech/homebrew-tap`
   with **Contents: read and write**. Prerelease tags skip the formula update.
4. To distribute macOS binaries, configure all five Apple secrets:
   `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`, `MACOS_NOTARY_KEY`,
   `MACOS_NOTARY_KEY_ID`, and `MACOS_NOTARY_ISSUER_ID`. Store the `.p12` and
   `.p8` file contents as base64 in the first and third secrets; the workflow
   passes those values to GoReleaser's cross-platform signing and notarization
   configuration. If any Apple secret is set, all five must be set. With none
   set, the release contains Linux and Windows artifacts only.
5. After the candidate is available on GitHub, open **Actions → Release → Run
   workflow** and select the candidate branch. The manual run builds the
   unsigned snapshot with the non-macOS configuration, validates the archive
   contents, and uploads a `relicta-release-snapshot` artifact. It does not
   create a tag or publish a GitHub Release, image, or package. Confirm the
   artifact is present and the validation job passed before creating a release
   tag. This exercises GitHub-hosted packaging, but does not test tag
   recommendation checks, reviewer protection, signing, or external publishing.

Never commit Apple keys, passwords, or tap credentials to the repository. Check
the Actions environment and secret names before the first release; a local
snapshot does not exercise reviewer protection or external publishing.

After the GitHub Release, verified container tags, and Homebrew formula update
finish, the workflow records their result with `relicta publish
--external-evidence-file <path>`. It binds the report to the approved tag and
commit and saves it in the release run. GitHub release and container
publication must succeed; Homebrew may be skipped for prereleases. A failed
publisher is recorded as a failed release run so `relicta status` and the
retained `relicta-release-outcome` workflow artifact show the failure and
successful partial work.

For other CI publishers, provide one JSON object with this shape:

```json
{
  "outcome": "published",
  "provider": "github-actions",
  "run_url": "https://github.com/owner/repository/actions/runs/123",
  "tag": "v1.2.3-rc.1",
  "commit": "<full commit SHA>",
  "publishers": {
    "github_release": {"status": "published", "url": "https://github.com/owner/repository/releases/tag/v1.2.3-rc.1"},
    "container": {"status": "published", "url": "https://ghcr.io/owner/repository", "digest": "sha256:<64 lowercase hex characters>"},
    "homebrew": {"status": "skipped"}
  }
}
```

Each publisher status is `published`, `failed`, or `skipped`. A `published`
result must include its HTTPS publisher URL. Successful reports require both
`github_release` and `container` to be `published`, include the container's
SHA-256 digest, and include Homebrew as `published` or `skipped` for a
prerelease. The GitHub Release and container publishers cannot be skipped.
Failed reports also require a top-level `failure` description and at least one
failed publisher. Relicta
checks the JSON shape and binds its tag and full commit to the approved run; it
does not independently query third-party publisher APIs, so workflows must
derive these statuses from their actual job results and protect the evidence
file from untrusted inputs. External finalization is limited to the built-in
tag step and optional attestation step; Relicta will not claim configured
plugins or other publishing steps ran when the external workflow did not run
them.

The repository also contains a plugin release workflow template. Plugin
releases have their own provenance and verification contract; do not assume
Relicta's release attestations cover artifacts published by a separate plugin
repository.

## Implementation map

| Control | Implementation |
| --- | --- |
| SHA-256 archive checksums | GoReleaser `checksum` in `.goreleaser.yaml` |
| Sigstore signature bundle for checksums | GoReleaser `signs` in `.goreleaser.yaml` |
| OIDC signing identity | `id-token: write` in `.github/workflows/release.yaml` |
| GitHub provenance for archives and checksums | `actions/attest-build-provenance` in `.github/workflows/release.yaml` |
| SBOM generation | GoReleaser `sboms` in `.goreleaser.yaml` |
