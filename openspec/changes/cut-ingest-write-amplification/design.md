## Context

The full measured background — prod table sizes, the TOAST bloat arithmetic, the
`pg_stat_user_tables` counters and the ruled-out I/O hypothesis — lives in
`docs/superpowers/specs/2026-08-02-ingest-write-amplification-design.md`. This document
covers only the technical decisions.

Two facts constrain every option below:

- The write path already knows whether a re-ingest changed anything. `UpsertJob` returns
  `changed` by comparing the incoming `content_hash` against the pre-update snapshot, and
  `cmd/ingest/store.go` already reads it to skip the Meili push. The signal exists; only
  Postgres does not act on it.
- The "refresh liveness without rewriting content" operation already exists as `TouchJob`,
  built for hydrating sources so a re-listed offer keeps its hydrated description. The seam
  this change needs is not new, it is unreached from the main path.

## Goals / Non-Goals

**Goals:**

- An ingest pass over a board of unchanged postings writes one narrow, HOT-eligible `UPDATE`
  per row and nothing to `companies`.
- `updated_at` comes to mean "content last changed", so `reindex --since` becomes genuinely
  incremental.
- Every observable behaviour is preserved: the 48h unseen sweep, the reopen, the incremental
  index push, the dedup marking, the enrichment enqueue.

**Non-Goals:**

- Reclaiming the ~43 GB already accreted (`pg_repack`). A repack before the write path is
  fixed just re-bloats; it is the follow-on change, gated on this one's measured effect.
- Dropping the two ~1 GB indexes with near-zero scans, purging `semantic_embedding`, tuning
  autovacuum workers, anything about Meilisearch's index size.

## Decisions

### The unchanged test lives in SQL, not in Go

`RefreshUnchangedJob` matches on `(source, external_id, content_hash)` and returns the row,
or nothing. The caller branches on `pgx.ErrNoRows`.

*Alternative considered:* read the stored hash into Go first, then choose a statement. That
costs a guaranteed extra round trip on the hot path and opens a window between the read and
the write. *Alternative considered:* extend the per-board seen-set (`ExistingExternalIDs`,
already one query per board returning `external_id → is_tech`) to carry `content_hash`, so
the pipeline could decide before writing. Attractive — it is a bulk read rather than a
per-row one — but the seen-set is consulted only by hydrating sources, so it would leave
most providers unchanged while adding a second decision site. Rejected as the wrong shape
for the first cut; the seam stays available.

The chosen form costs **one** statement on the common path, the same as today. Only the
rarer changed path pays two.

### The cheap branch returns the same row type

The helper returns a `db.UpsertJobRow` either way, with `Inserted` and `Changed` false on
the cheap branch. The remainder of `save` is then untouched: `clustersByRole` and
`needsIndex` are already gated on exactly those two fields and already evaluate false for an
unchanged row. The dedup lookup and the index push are therefore skipped by the logic that
already exists, not by a new branch someone must keep in sync.

This is also why the apply-form write and the crawled-set record need no special handling —
they sit after the seam and run on both branches, as they must.

### `closed_at IS NULL` is a correctness predicate, not an optimisation

Without it, a posting that had been closed and reappears on the board with identical content
would have its liveness refreshed and stay closed — the sweep would then never reopen it,
because it is being seen. Excluding closed rows sends them to `UpsertJob`, which reopens.

### The enrichment enqueue stays on the cheap path

`EnqueueJobEnrichment` is already gated (`enriched_at IS NULL OR enrichment_version <
target`) and idempotent on `UNIQUE (job_id, target_version)`. Keeping it preserves today's
behaviour that a job which never got enriched is re-offered on every crawl. Dropping it
would save a lookup per row but would strand such a job until a manual backfill — a worse
trade than the lookup costs.

### Skipping derived columns is safe by subset, and the subset is tested

`RoleFingerprint` reads `company_slug`, `title`, `description`; all three are inputs to
`jobhash.Of`. Equal `content_hash` therefore implies equal `role_fingerprint`. The same
argument covers the deterministic facets. This is an invariant between two functions that
can drift, so it becomes a test in `internal/jobhash` rather than a comment — the package
already has precedent for exactly this (`TestOfRow_CarriesEveryFieldTheHashReads` exists
because the same mapping had already drifted once).

### Only `UpsertJob`'s company CTE is guarded

Three queries carry a `company_upsert` CTE. `UpsertManualJob` and `UpdateManualJob` fire
once per moderator action; guarding them would be churn without measurable value. The guard
goes on `UpsertJob`'s alone, which is the one running millions of times per day.

## Risks / Trade-offs

**The cheap-path hit rate is unmeasured and may be low for some providers** → `jobhash.Of`
covers 19 fields, and a provider only has to vary one per crawl — a session token in the
URL, a re-serialized `posted_at`, unstable whitespace in `location` — for its rows to keep
taking the full path forever. The failure is silent, the same shape as `ingested=0 failed=0`
on a dead board. Mitigation: per-provider hit-rate logging is in scope (not deferred), and
the first post-release check is that distribution rather than the global counters. A
provider at ~0% is a finding worth acting on at the adapter, since the same churn has also
been forcing a pointless index push every crawl.

**The `companies` guard saves regardless, which could mask a failed `jobs` fix** → the two
effects must be read separately in `pg_stat_user_tables`, not as one aggregate number. The
verification step names both tables for this reason.

**`fillfactor = 90` does nothing to existing pages** → they stay packed at 100% until the
deferred repack, so the HOT share improves gradually rather than at once. This is expected,
not a defect; it means the 24h measurement will understate the eventual steady state.

**The storage-parameter migration takes an ACCESS EXCLUSIVE lock** → no table rewrite, but
it will queue behind a long-running read and block every reader behind it, the same
mechanism that has bitten `ADD CONSTRAINT` here before. Mitigation: its own migration file,
deployed separately with a `lock_timeout`, retried rather than waited out.

**Falling back on `ErrNoRows` conflates "changed" with "absent"** → a brand-new posting also
returns no rows, and correctly falls through to `UpsertJob`, which inserts. The conflation is
harmless because both cases want the same statement; noted so a future reader does not
"fix" it into a separate existence check.

## Migration Plan

1. Snapshot `pg_stat_user_tables` for `jobs` and `companies` on prod before the release.
2. Release the code change. It is backward-compatible: no schema dependency, and a rollback
   is a plain revert with no data migration.
3. Deploy the storage-parameter migration on its own, with `lock_timeout`.
4. After 24 h, re-snapshot. Expected: `companies.n_tup_upd` down by orders of magnitude,
   `jobs.n_tup_upd` down toward the rate of genuine content change, HOT share up, dead
   tuples falling. Read the per-provider hit-rate logs first.

If the counters do not move, the diagnosis was wrong and the follow-on repack must not be
run on this change's strength.

## Open Questions

None blocking. Deferred by decision, not by uncertainty: whether the per-board seen-set
should later carry `content_hash` so the decision moves out of the per-row statement
entirely.
