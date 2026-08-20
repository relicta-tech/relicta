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
