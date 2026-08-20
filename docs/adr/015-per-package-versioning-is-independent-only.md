# ADR-015: Per-Package Versioning Is Independent Only

## Status

Accepted (2026-08-20)

## Date

2026-08-20

## Context

The `monorepo:` section was the largest unread block in the configuration, and the
subsystem behind it the largest unreached code in the tree. Every field — `enabled`,
`strategy`, `package_paths`, `exclude_paths`, `package_overrides`, `version_files`,
`release_groups` — had zero production readers, while `internal/domain/monorepo` and
`internal/application/monorepo` held roughly 3,000 lines of implemented, tested code
that nothing in the release path called, plus another 3,100 lines of tests for it.

Verified against the built binary. A repository with `enabled: true`,
`strategy: independent`, `package_paths: ["packages/*"]` and two packages at 1.0.0 and
2.0.0:

    Current version:  0.0.0
    Next version:     0.1.0

One repository-wide version, and neither `package.json` touched.

The distinction that made this hard to see: package *analysis* worked. `relicta blast`
found both packages and the one affected — but it reads `blast_radius`, not `monorepo`.
A monorepo user was not wrong that Relicta understood their layout. They were wrong
about which part of it versioned their packages.

Leaving it was not an option; both remaining options had a real cost. Deleting ~6,200
lines would remove working analysis, orchestration and version writers. Wiring all of it
meant answering, at once, how a monorepo release interacts with the single `ReleaseRun`
aggregate, with tags, with the changelog, and with a governance record that is currently
one decision per release rather than one per package.

## Decision

**`strategy: independent` is wired. `lockstep`, `hybrid` and `release_groups` are
refused at config load.**

Each package's own commits decide its own bump, read from and written to its own
manifest. `relicta bump` answers per package; `plan`, `publish` and `release` still act
on the repository as a whole and now say so on every run.

**The unimplemented strategies are errors, not warnings.** While the whole section did
nothing, a warning was right: the release still behaved as the repository-wide config
said it would. That is no longer true. The same file now gets per-package versioning for
one strategy and repository-wide versioning for the others, and handing somebody who
asked for lockstep the opposite of lockstep is worse than refusing to start.

**A package is typed by the manifest in its own directory.** The workspace model carries
one `PackageManager` for the whole workspace, which is right for a pnpm or Cargo
workspace and wrong for the layout `package_paths` exists to describe: `packages/*` may
hold an npm package beside a Go module. Typing every package from the workspace would
read one manifest and write another. The workspace manager remains the fallback.

**The base version comes from the last release, not from the working tree.** Reading the
manifest on disk means reading the file the previous bump wrote, so bumping twice
without releasing took a package from 2.1.3 to 3.0.0 and then to 4.0.0 off one commit.
The manifest is read as it stood at the base ref, which makes a second run report what
the first did — the same property the repository-wide path gets from its tag.

**Manifest edits are surgical.** A bump changes one value; key order, indentation and
comments belong to the project.

## Consequences

Three defects in the version writers had to be fixed before they could ship, all of
which only a real repository would have shown:

- `package.json` was decoded into a `map[string]interface{}` and re-marshalled. Go
  marshals map keys alphabetically, so every bump would have permanently reordered the
  manifest, moving `name` below `dependencies`.
- `Cargo.toml` and `pyproject.toml` were rewritten with `^(\s*version\s*=\s*)"[^"]+"`
  applied to the whole file. A dependency declared as its own table —

      [dependencies.serde]
      version = "1.0"

  is a line beginning with `version =`, so the package's version was written over the
  dependency's. Silent corruption of a file the build reads.
- Manifests were rewritten `0644`, widening a file somebody had deliberately kept
  private.

That they were all in unreachable code is the point: tested code that nothing calls
attracts exactly this. The tests passed on inputs the tests chose.

What is not done, and is named as not done wherever a user would meet it: per-package
tags (`api-v1.5.0`), per-package changelogs, and a governance decision per package
release. Until those exist, `relicta bump` measures every package from the repository's
last tag, because there is no `api-v1.4.0` to measure from.

Nothing changes for a repository without `monorepo.enabled`, which is every repository
that has one today, since the section was read by nothing.

## Amendment: per-package tags (2026-08-20)

Each package now carries its own tag — `api-v1.5.0` by default, from the package's directory
name, or whatever `monorepo.package_overrides.<path>.tag_prefix` says. `relicta publish`
creates one per package at the version that package's manifest claims, and `relicta bump`
reads them back to measure each package from its own last release rather than from the
repository's.

**Alongside the repository's tag, not instead of it.** The repository tag stays the marker the
repository-wide commands measure from; a monorepo with none would have every one of them
counting from the start of history forever. It is also what the governance record is anchored
to, which remains one decision per release.

Three things this closed:

- The release commit now covers the packages' manifests. Without them the tag pointed at a
  commit that did not contain the versions it claimed, and the clean-tree gate refused the
  publish outright — reproduced against the shipped binary.
- `relicta bump` in a monorepo left the release run in `planned`, so `notes`, `approve` and
  `publish` all refused with "run 'relicta bump' first" to somebody who just had. Per-package
  versioning replaces the repository's manifest version, not its release record.
- Measuring each package from its own tag makes bumping idempotent for any package that has
  been released once. A package that never has still compounds if bumped repeatedly before its
  first release, because there is nothing yet to measure from.

Still one per repository, and still said out loud on every run: the plan and the governance
decision. Per-package changelogs and approvals are the next slice.

## Amendment: per-package changelogs (2026-08-20)

Each package now gets its own `CHANGELOG.md` — or whatever
`monorepo.package_overrides.<path>.changelog_file` names — written during `publish` and
carried by the release commit, so the package's tag contains the entry describing it.

**Rendered from the package's own commits, not from AI notes.** The repository's changelog is
written from the release notes, which are generated once for the release as a whole. A package's
entry is built from that package's conventional commits, through the same renderer and the same
`changelog.*` settings, so the two files cannot drift into different formats in one repository.
It is also free and deterministic: a per-package changelog that needed an API key would make
monorepo releases fail for everyone without one.

**The heading is the version being tagged.** By publish time `bump` has already written the
manifests, so a version recomputed at that point is the one *after* this release — the first
draft printed `## [1.6.0]` above a release tagged `api-v1.5.0`. The commits come from the
analysis, the version from the manifest, and they are joined.

Three defects fell out of it, all of them in code paths that could not be reached before:

- The `monorepo:` section had no viper defaults registered, so a config naming only `enabled`
  and `package_paths` loaded with an empty strategy and was refused by the validation this ADR
  introduced. Defaults are now set per key, as `persistence` already does.
- The analyzer passed the raw git subject into the changeset, so entries read
  `- fix: correct the status code` under a `### Bug Fixes` heading.
- Its fallback classifier was a hand-rolled prefix match over five commit types that dropped
  the scope entirely, read `perf`, `build`, `ci`, `style` and `revert` as chores, and
  classified `fixup! ...` as a fix. It now uses the domain's conventional-commit parser, which
  is what the rest of the release path has always used.

## Amendment: a decision per package (2026-08-21)

Each package now carries its own release run — its own version, notes, risk assessment,
approval and audit entry. `relicta approve --package <name>` decides one; `relicta publish`
tags only the packages that were decided, so a package held back does not ship.

```
relicta approve --package api    → api  1.5.0  approved
relicta publish                  → Created tag v1.0.0; created package tags api-v1.5.0

Package decisions
  packages/api    1.5.0   approved
  packages/web    3.0.0   notes_ready — held, will not be tagged
```

**A monorepo release has two levels, and both are decisions.** `relicta approve` decides the
release itself — the repository's run, which carries the release-level governance and the
marker every repository-wide command measures from. `--package` decides what is in it. Neither
implies the other: approving the release does not ship a package nobody looked at, and approving
a package does not start a release.

**The run's identity carries the package.** A run's ID is derived from its plan hash, which
covers repoID, base ref, head SHA, commits and version — and for two packages of one repository
every one of those can match. The first working version produced this:

    packages/api    1.5.0    run-862edbf3
    packages/web    3.0.0    run-862edbf3

Three runs, one ID: three decisions that could not be told apart, in a tool whose product is the
audit trail. The package is now part of `repoID` — `git@github.com:org/repo.git#packages/api` —
rather than a new persisted field, and deliberately so. Including `repoRoot` in the hash
unconditionally would give every existing repository new IDs, and the next `plan`, which
supersedes runs whose hash no longer matches, would cancel an in-flight approved release on
upgrade. A new field would need a column in two SQL backends and a migration for a distinction
`repoID` already expresses: the releasable unit is this repository, this package.

**A package's run records the package's commits.** `PlanReleaseInput` gained `Commits`, used
instead of resolving the whole range. Without it each package's run listed every commit between
base and HEAD, so its risk was assessed on changes it does not contain and its record claimed
work another package did.

Still open, and named in the backlog rather than faked: a package's run stays at `approved`
after its tag is created. `MarkPublished` requires the publishing state and a completed step
plan, so moving it there means running each package's own publish — a per-package step plan, not
a status write.
