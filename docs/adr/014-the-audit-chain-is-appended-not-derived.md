# ADR-014: The Audit Chain Is Appended, Not Derived

## Status

Accepted (2026-08-18)

## Date

2026-08-18

## Context

`internal/cgp/audit` has shipped a hash-linked, append-only chain since the CGP work
landed — SHA-256 over each entry plus its predecessor's hash, `Append`, `Verify`,
`LastHash`, `Len` — documented as "an immutable, verifiable record of all governance
decisions, enabling tamper detection and compliance verification".

It was constructed once, inline, at `internal/container/container.go`:

```go
WithAuditChain(audit.NewChain())
```

Nothing in production ever called `Append`, nothing ever called `Verify`, and the chain
was in memory only — no save, no load. It was handed to the attestation generator, whose
`populateAuditChain` reads `LastHash()` and `Len()`.

Decoded from a real `attestation.intoto.jsonl` after a full release with
`attestation.enabled: true`:

```
auditChainHash  = ''
auditEntryCount = 0
```

Two fields in a signed supply-chain attestation that assert a governance audit chain and
certify an empty one. Downstream, `internal/integrations/drata/mapping.go` and
`internal/integrations/vanta/mapping.go` both map `AuditChainHash` to `IntegrityHash` and
push it to a compliance platform, and `internal/compliance/annex_iv.go` describes the
control as *"SHA-256 hash chain over JSON-canonical CGP decisions; integrity verified at
report-generation time"*. Every one of those statements was true of a chain with nothing
in it.

Making the fields true requires answering two questions, and the second is the one that
decides the design.

## Decision

### The chain is appended at the time of each event, not derived from the records

Both were available. The governance memory store already holds `GovernanceDecision` and
`ExecutionAuthorization` records, so a chain could be a projection over them: sort, hash,
link, done. No new storage, no new writes, and every existing record retroactively covered.

**We reject the derived chain, because a hash chain over mutable records detects nothing.**

Every backend's `RecordDecision` is an upsert. SQLite:

```sql
INSERT INTO governance_decisions (...) VALUES (...)
ON CONFLICT(decision_id) DO UPDATE SET ...
```

with the comment "replacing one already recorded under its ID — which is what assigning
into the reference's map does". That is correct behavior for a record a later run can
correct, and it is fatal for evidence. Edit a risk score, recompute the projection, and the
derived chain re-links around the edit and reports itself intact. It is a checksum of the
present state, not a record of the past — it can tell you the projection is
self-consistent, which is a property of the code that built it rather than of the history
it claims to attest to.

The appended chain makes the opposite trade. The entry is written once, at the moment the
event happens, hashing its own content and its predecessor's hash. Nothing later can change
either without every entry after it failing verification — which a test proves by actually
corrupting a stored entry and asserting that `Verify` names it. The cost is a second copy
of facts the decision record already holds, and that duplication *is the point*: the value
of the second copy is precisely that it cannot be revised to match the first. When the two
disagree, the chain is the one that was written at the time.

The tension is real and worth stating plainly. The derived chain would have covered every
release already in the store; the appended one starts empty and accumulates from the first
release after upgrade. We accept an empty chain today over a chain that would have vouched
for records it cannot actually vouch for.

### It lives behind the governance memory port, not in a store of its own

ADR-013 put one port set behind `persistence.backend` and named "five stores that each
invented their own persistence" as the defect. A chain in a file beside `memory.json` would
have been the sixth, and the specific failure it would cause is easy to name: an operator
who set `backend: sqlite` would move their governance records into the database and leave
their evidence behind in a file the new backend never reads.

So `memory.Store` embeds a three-method `audit.Store`:

```go
AppendAuditEntry(ctx, repository string, entry *Entry) error
LastAuditEntry(ctx, repository string) (*Entry, error)
AuditChain(ctx, repository string) ([]*Entry, error)
```

One resolver (`persistence.OpenGovernanceMemoryStore`), the same four adapters — file,
in-memory, SQLite, PostgreSQL — and the same conformance suite, extended with nine cases
that are the whole of the append-only contract.

**We reject making `ports.EventStore` the home**, and this resolves the question ADR-013
left open rather than sidestepping it. ADR-013 decided the event log "is not the system of
record" and that "events remain a publication mechanism for webhooks and subscribers". An
audit chain is the system of record for what governance did; putting it in the event log
would make the event log authoritative for one thing and advisory for everything else,
which is exactly the third copy of the truth ADR-013 refused. The chain reads the event
stream — the `EventRecorder` decorator sits in the publisher chain — but it writes to the
governance store, so events stay a transport and the record stays in one place.

### Linking is one definition; storage enforces one precondition

`audit.Recorder` reads the stored tail, sets `PreviousHash`, computes `Hash` and appends.
Every backend gets fully-linked entries, so there is one hashing implementation rather than
four.

What each backend must enforce is that the entry follows the chain *as stored right now*:
`ErrChainForked` when `PreviousHash` is not the current tail's hash, `ErrDuplicateEntry`
when the ID is already present. Two concurrent writers otherwise both read one tail and
both insert, producing two chains that each verify and disagree about what happened, with
nothing in the data to adjudicate. SQLite takes an immediate transaction; PostgreSQL uses a
serializable transaction *and* a `UNIQUE (repository, previous_hash)` constraint, so a fork
is refused by the schema even if a future caller forgets the transaction.

### What is recorded

Six transitions, in two places, because no single seam sees them all.

From the release event stream, via a decorator beside the outcome tracker — `release.created`
and `release.versioned` become `proposal.received`, `release.approved` becomes
`approval.granted` with its actor, `release.published` becomes `execution.completed`, and
`release.failed` becomes `execution.failed`. The event stream is used because `relicta` is
several processes: plan, approve and publish are separate invocations, and the run ID on the
event is what ties them together.

From `governance.Service.EvaluateRelease` — `evaluation.completed` (what the risk model
found) and `decision.made` (what governance concluded). Two entries because they can
differ: the reputation guard downgrades an auto-approval after the score is computed. This
is also the only place a **rejection** becomes evidence: a rejected release raises no
domain event at all, so a chain built only from the event stream would contain every
approval and no refusal.

`release.canceled` is deliberately absent — CGP Section 9 has no term for it, the outcome
tracker already records it as a canceled release, and inventing a term here is how the
chain and `relicta audit` come to tell different stories.

### A break is loud

`relicta verify` now checks two things beyond the signature. That the repository's chain
verifies, and that it still confirms the position the attestation was sealed over: entry
`auditEntryCount - 1` must still hash to `auditChainHash`. Both failures exit non-zero and
name the entry. `relicta audit` reports the chain's length and integrity beside the release
history it prints.

The anchor check is what makes rebuilding the chain useless. A chain recomputed from
scratch verifies perfectly; it just no longer contains the entry the attestation was signed
over.

## Consequences

**The two attestation fields become true.** Verified against the built binary: three
releases in a throwaway repository produced a 24-entry chain and attestations anchored at
entries 7, 15 and 23. Editing one entry's risk score, deleting an entry, and truncating the
chain each fail `relicta verify` with exit 1 and the entry named.

**The chain starts empty and does not backfill.** Releases published before this change
carry `auditChainHash: ""`, which `relicta verify` reports as *"this attestation records no
chain position"* — a warning, not a failure. Treating them as tampered would fail every
historical release on upgrade; treating them as verified would be the original lie.

**Governance memory becomes load-bearing.** With `governance.memory_enabled: false` there is
no chain and the attestation reports an empty one, honestly. That is a real reduction in
what an operator can turn off without losing something, and it is the correct direction for
a tool whose product is audit evidence.

**Recording is best effort; forwarding is not.** A failed append is logged and does not
block the release, matching the outcome tracker beside it — a full disk should not block a
release, and relicta's evidence is not worth more than the release it is evidence of. The
loss is visible rather than silent: the attestation's entry count stops matching the
release history, and `relicta audit` prints both.

**Two backends now have a table nothing had before.** SQLite migration 004 and PostgreSQL
migration 004. `persistence.migration_mode: manual` databases must run `relicta db migrate`;
`VerifyGovernanceMemorySchema` now probes the chain table too, so such a database is refused
at open with the missing relation named rather than failing at the first append, mid-release.

**The file backend gains a ceiling.** `memory.json` is one document read under a 5 MB
limit, and a chain entry costs roughly 550 bytes — about eight entries per release, so the
limit arrives somewhere near a thousand releases, and reaching it fails the whole store
rather than only the chain. Deliberately not raised: a larger number moves the wall rather
than removing it, and changes what every existing `memory.json` is allowed to grow to.
ADR-013 already names SQLite as the destination for a repository with real history.

**`relicta verify` reads configuration now.** It was in the config-skip list, which was
harmless while it only read a file. It needs `persistence.backend` to find the chain, and
tolerantly: verifying an attestation downloaded from a release page, with no repository
around it, is what the command is for, and the chain is then reported as unchecked.
