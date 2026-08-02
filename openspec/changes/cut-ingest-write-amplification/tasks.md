## 1. Guard the invariant the whole change rests on

- [x] 1.1 Add a test in `internal/jobhash` asserting every input `RoleFingerprint` reads is
      also an input `jobhash.Of` reads, so a fingerprint can never go stale behind a matching
      `content_hash`. Follow the shape of the existing `TestOfRow_CarriesEveryFieldTheHashReads`.
- [ ] 1.2 The same guard for the deterministic facets, which the cheap path also skips: a test
      asserting that if mutating a `jobderive.Input` field moves any `Derived` value, that field
      is also a `jobhash.Of` input. Three `Input` fields are known not to be hashed — `Source`
      and `ExternalID`, safe because `RefreshUnchangedJob` matches a row BY them so they cannot
      move, and `Cities`, which is not safe. Carry the safe two as named, reasoned exemptions the
      test states rather than a blanket allowance, and report what `Cities` turns out to be: if a
      structured-source city list can change while every hashed field stays put, the cheap path
      must not skip that row. Resolve it here — by hashing the field, or by narrowing the
      predicate — rather than deferring it into the write path.

## 2. The cheap write

- [ ] 2.1 Add `RefreshUnchangedJob` to `internal/db/queries/jobs.sql`: `UPDATE jobs SET
      last_seen_at = now()` matched on `(source, external_id, content_hash)` and guarded by
      `closed_at IS NULL`, returning the row. Run `make sqlc`.
- [ ] 2.2 Integration test (`internal/db`, `//go:build integration`): the query advances
      `last_seen_at`, leaves every other column including `updated_at` untouched, and returns
      no row for a closed posting or a mismatched hash.

## 3. The ingest seam

- [ ] 3.1 In `cmd/ingest/store.go`, extract the `qtx.UpsertJob` call in the private `save`
      behind a helper that tries `RefreshUnchangedJob` first and falls back on
      `pgx.ErrNoRows`, returning a `db.UpsertJobRow` with `Inserted`/`Changed` false on the
      cheap branch. `Save` and `SaveWithApplyForm` both inherit it.
- [ ] 3.2 Integration test: re-ingesting an identical posting reports neither inserted nor
      changed, issues no index push and no role-cluster lookup, and still records the company
      into the crawled-set and runs the enrichment enqueue.
- [ ] 3.3 Integration test: re-ingesting a **closed** posting with identical content reopens
      it — `closed_at` and the close record cleared, strikes reset, `updated_at` stamped. This
      is the regression guard for the `closed_at IS NULL` predicate.
- [ ] 3.4 Integration test: `SaveWithApplyForm` on an unchanged posting still writes the
      apply form.

## 4. `updated_at` means "content changed"

- [ ] 4.1 Change `TouchJob` (`jobs.sql`) to stamp `updated_at` only when it reopens:
      `updated_at = CASE WHEN closed_at IS NOT NULL THEN now() ELSE updated_at END`.
      Run `make sqlc`.
- [ ] 4.2 Integration test: `TouchJob` on an open row advances `last_seen_at` and leaves
      `updated_at`; on a closed row it reopens and stamps `updated_at`.

## 5. The company row

- [ ] 5.1 Guard the `company_upsert` CTE inside `UpsertJob` only with
      `WHERE companies.name IS DISTINCT FROM EXCLUDED.name`. Leave the CTEs in
      `UpsertManualJob` and `UpdateManualJob` alone. Run `make sqlc`.
- [ ] 5.2 Integration test: ingesting a posting for an existing company under an unchanged
      name leaves `companies.updated_at` untouched; a renamed company still updates both
      `name` and `updated_at`.

## 6. Make the reach observable

- [ ] 6.1 Count cheap-path versus full-path writes per ingest run and log the pair once at
      run end, attributed to the provider. Wire the counter through the existing run summary
      rather than adding a parallel reporting path.
- [ ] 6.2 Test: a run that re-saw only unchanged postings reports a full cheap-path share; a
      run whose postings all changed reports zero, rather than reporting nothing.

## 7. Storage parameters

- [ ] 7.1 New migration setting `fillfactor = 90` and `autovacuum_vacuum_scale_factor = 0.02`
      on `jobs`, in its own file so it deploys separately. Confirm the next free migration
      number against prod, not just against the repo.

## 8. Verify

- [ ] 8.1 `go build ./... && go vet ./... && go test ./...`, then
      `go vet -tags=integration ./...` and `go test -tags=integration ./internal/db/ ./cmd/ingest/`.
- [ ] 8.2 Snapshot `pg_stat_user_tables` for `jobs` and `companies` on prod and record the
      numbers in the change before releasing, so the post-release comparison has a baseline.
