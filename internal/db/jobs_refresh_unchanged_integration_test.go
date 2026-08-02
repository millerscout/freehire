//go:build integration

// Integration tests for the cheap ingest write: RefreshUnchangedJob advances last_seen_at on a
// re-seen posting that matches the stored row on the cheap path's key, writing nothing else —
// not even updated_at, which is what makes `reindex --since` incremental. It must match nothing
// when the content hash moved, when the structured cities moved, or when the row is closed, so
// each of those falls through to the full upsert. All of it is SQL behaviour verifiable only
// against a real Postgres. Run with: go test -tags=integration ./internal/db/
package db

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// unchangedParams is a re-crawl of the posting seedForRefresh wrote: same identity, same hash,
// same cities. Each case below varies exactly one of those to prove it is part of the match.
func unchangedParams() RefreshUnchangedJobParams {
	return RefreshUnchangedJobParams{
		Source:      "greenhouse",
		ExternalID:  "acme:1",
		ContentHash: pgtype.Text{String: "h1", Valid: true},
		Cities:      []string{"berlin"},
	}
}

// seedForRefresh ingests one open posting and back-dates its liveness and change stamps, so a
// refresh that touches either is visible as a move rather than as two equal timestamps.
func seedForRefresh(t *testing.T, pool *pgxpool.Pool, q *Queries) Job {
	t.Helper()
	ctx := context.Background()

	p := ingestParams("acme:1", "Engineer")
	p.ContentHash = pgtype.Text{String: "h1", Valid: true}
	p.Cities = []string{"berlin"}
	if _, err := ingestUpsert(ctx, q, p); err != nil {
		t.Fatalf("seed upsert: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE jobs SET last_seen_at = now() - interval '10 days', updated_at = now() - interval '10 days'
		 WHERE source = 'greenhouse' AND external_id = 'acme:1'`,
	); err != nil {
		t.Fatalf("back-date stamps: %v", err)
	}

	got, err := q.GetJobBySourceExternalID(ctx, GetJobBySourceExternalIDParams{Source: "greenhouse", ExternalID: "acme:1"})
	if err != nil {
		t.Fatalf("seed readback: %v", err)
	}
	return got
}

func TestRefreshUnchangedJobWritesOnlyLastSeenAt(t *testing.T) {
	pool := startPostgres(t)
	q := New(pool)
	ctx := context.Background()
	truncate(t, pool)

	before := seedForRefresh(t, pool, q)

	row, err := q.RefreshUnchangedJob(ctx, unchangedParams())
	if err != nil {
		t.Fatalf("RefreshUnchangedJob: %v", err)
	}
	got := row.Job

	if !got.LastSeenAt.Time.After(before.LastSeenAt.Time) {
		t.Errorf("last_seen_at = %v, want advanced from %v", got.LastSeenAt.Time, before.LastSeenAt.Time)
	}
	if time.Since(got.LastSeenAt.Time) > time.Minute {
		t.Errorf("last_seen_at = %v, want ~now", got.LastSeenAt.Time)
	}
	// The point of the query: the change stamp does NOT move, so an incremental reindex scoped
	// by updated_at does not re-push a row whose content stood still.
	if !got.UpdatedAt.Time.Equal(before.UpdatedAt.Time) {
		t.Errorf("updated_at = %v, want unchanged %v", got.UpdatedAt.Time, before.UpdatedAt.Time)
	}

	// Nothing else moved either. Compared as whole rows with the one column the query is allowed
	// to write zeroed, so a column added to jobs later is covered without editing this.
	a, b := before, got
	a.LastSeenAt, b.LastSeenAt = pgtype.Timestamptz{}, pgtype.Timestamptz{}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("row changed beyond last_seen_at:\n before = %+v\n after  = %+v", a, b)
	}
}

func TestRefreshUnchangedJobMatchesNothingWhenTheRowWouldChange(t *testing.T) {
	pool := startPostgres(t)
	q := New(pool)
	ctx := context.Background()

	movedHash := unchangedParams()
	movedHash.ContentHash = pgtype.Text{String: "h2", Valid: true}

	movedCities := unchangedParams()
	movedCities.Cities = []string{"munich"}

	cases := map[string]struct {
		params RefreshUnchangedJobParams
		closed bool
	}{
		// A moved hash means the posting's indexed content changed: the full upsert must run.
		"content hash moved": {params: movedHash},
		// cities is the one written column the content hash does not cover, so it is matched on
		// separately — see TestUpsertParams_CheapWriteMatchKeyCoversEveryColumnItWrites.
		"structured cities moved": {params: movedCities},
		// A closed row must reach UpsertJob, which is what reopens it. Refreshing its liveness
		// here would leave it closed forever while the unseen sweep kept seeing it.
		"row is closed": {params: unchangedParams(), closed: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			truncate(t, pool)
			seedForRefresh(t, pool, q)
			if tc.closed {
				if _, err := pool.Exec(ctx,
					`UPDATE jobs SET closed_at = now() WHERE source = 'greenhouse' AND external_id = 'acme:1'`,
				); err != nil {
					t.Fatalf("close: %v", err)
				}
			}

			if _, err := q.RefreshUnchangedJob(ctx, tc.params); !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("err = %v, want pgx.ErrNoRows so the caller falls through to UpsertJob", err)
			}
		})
	}
}
