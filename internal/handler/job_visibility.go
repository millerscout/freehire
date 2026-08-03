package handler

import "github.com/strelov1/freehire/internal/db"

// jobVisibleTo reports whether job is visible to the given caller. A public job is
// visible to everyone, authenticated or not. A private job (jobs.is_private — the
// jd-tailor-intake path: a pasted JD or an unrecognized-URL scrape) is visible only to
// its creator; anyone else, including an anonymous caller, must see it exactly as if the
// slug did not exist at all (see the job-public-identity, job-fit-analysis, and
// cv-tailoring capability deltas in openspec/changes/jd-tailor-intake).
func jobVisibleTo(job db.Job, callerID int64, authenticated bool) bool {
	if !job.IsPrivate {
		return true
	}
	return authenticated && job.CreatedBy.Valid && job.CreatedBy.Int64 == callerID
}
