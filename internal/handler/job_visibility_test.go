package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/strelov1/freehire/internal/db"
)

func TestJobVisibleTo_PublicJobIsVisibleToAnyone(t *testing.T) {
	job := db.Job{IsPrivate: false}
	if !jobVisibleTo(job, 0, false) {
		t.Error("public job must be visible to an anonymous caller")
	}
	if !jobVisibleTo(job, 42, true) {
		t.Error("public job must be visible to any authenticated caller")
	}
}

func TestJobVisibleTo_PrivateJobIsVisibleOnlyToItsCreator(t *testing.T) {
	job := db.Job{IsPrivate: true, CreatedBy: pgtype.Int8{Int64: 7, Valid: true}}
	if !jobVisibleTo(job, 7, true) {
		t.Error("private job must be visible to its creator")
	}
	if jobVisibleTo(job, 8, true) {
		t.Error("private job must not be visible to a different authenticated caller")
	}
	if jobVisibleTo(job, 0, false) {
		t.Error("private job must not be visible to an anonymous caller")
	}
	if jobVisibleTo(job, 7, false) {
		t.Error("private job must not be visible when the caller id matches but the request is unauthenticated")
	}
}

func TestJobVisibleTo_PrivateJobWithNoCreatedByIsVisibleToNoOne(t *testing.T) {
	// Defensive: created_by should always be set on a private row, but a NULL one must
	// never accidentally match any caller.
	job := db.Job{IsPrivate: true}
	if jobVisibleTo(job, 0, true) {
		t.Error("a private job with no recorded creator must not be visible to anyone")
	}
}
