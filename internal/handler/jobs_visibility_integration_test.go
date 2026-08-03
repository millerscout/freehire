//go:build integration

// Integration test for GET /api/v1/jobs/:slug's private-job gate: a private job
// (jd-tailor-intake) not owned by the caller — including an anonymous one — is answered
// exactly as if the slug did not exist. Run with: go test -tags=integration ./internal/handler/
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/strelov1/freehire/internal/auth"
	"github.com/strelov1/freehire/internal/db"
)

func TestGetJob_PrivateJobGate(t *testing.T) {
	pool := startPostgres(t)
	ctx := context.Background()
	queries := db.New(pool)

	var owner, stranger int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('getjob-owner@example.test') RETURNING id`).Scan(&owner); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email) VALUES ('getjob-stranger@example.test') RETURNING id`).Scan(&stranger); err != nil {
		t.Fatalf("seed stranger: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO jobs (source, external_id, url, title, public_slug, is_private, created_by)
		 VALUES ('pasted', 'gj1', '', 'A private JD', 'getjob-private-job', true, $1)`, owner); err != nil {
		t.Fatalf("seed private job: %v", err)
	}

	iss := auth.NewIssuer("test-secret", time.Hour)
	ownerTok, _ := iss.Issue(owner, testTokenVersion)
	strangerTok, _ := iss.Issue(stranger, testTokenVersion)

	h := newJobsHandlers(queries, nil)
	app := fiber.New(fiber.Config{ErrorHandler: RenderError})
	app.Get("/api/v1/jobs/:slug", auth.OptionalAuth(iss, testVersions, apiKeys{queries}), h.GetJob)

	get := func(tok string) int {
		req := httptest.NewRequest(fiber.MethodGet, "/api/v1/jobs/getjob-private-job", nil)
		if tok != "" {
			req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: tok})
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("GET job: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if status := get(""); status != fiber.StatusNotFound {
		t.Errorf("anonymous GetJob = %d, want 404", status)
	}
	if status := get(strangerTok); status != fiber.StatusNotFound {
		t.Errorf("stranger GetJob = %d, want 404", status)
	}
	if status := get(ownerTok); status != fiber.StatusOK {
		t.Errorf("owner GetJob = %d, want 200", status)
	}
}
