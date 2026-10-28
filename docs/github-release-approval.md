# GitHub release approval

Relicta's `release.yaml` workflow separates planning from publishing. The plan job
creates the Relicta release run and writes its version, tag, commit, and run ID to
the Actions summary. The publish job then waits on the `release-approval`
environment before it can approve the Relicta run or publish artifacts.

## Configure the environment

In the repository settings, open **Environments**, create `release-approval`,
and configure:

- **Required reviewers**: add the people or teams allowed to review releases.
- **Prevent self-review**: enable this so the person who triggered the run cannot
  approve their own deployment.

Keep the environment name in sync with `environment:` in
`.github/workflows/release.yaml`. GitHub only pauses the publish job when the
environment has required protection rules. Relicta also queries the workflow
run's approval history and stops before publication if the exact environment has
no approved reviewer record.

## Audit and permissions

The workflow grants the publish job `actions: read`; Relicta uses its
`GITHUB_TOKEN` to verify the current run's GitHub-reported SHA and the approved
environment/reviewer record. It stores that response's environment, reviewer
login(s), run URL, ref, and SHA in the release approval and approval event. The
CI actor remains recorded as CI; the human reviewer is separate evidence.

Successful external publication verifies the reviewer record again before
Relicta accepts the publisher's evidence. A failure outcome can still be
recorded so failed attempts remain visible in the release history.

The verifier currently accepts GitHub.com (`api.github.com`) only. It refuses
custom API origins so a supplied API URL cannot redirect the workflow token to
an untrusted host.
