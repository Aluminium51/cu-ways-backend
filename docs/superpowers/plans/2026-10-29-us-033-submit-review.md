# US-033 Submit Review Implementation Plan

**Goal:** Let the Creator who owns a completed Job submit one 1–5 star review with an optional comment.

**Architecture:** Add `POST /api/v1/jobs/{id}/reviews` using the existing Fiber handler → service → port → PostgreSQL repository structure. The service validates Creator membership and rating; the repository checks Job ownership and Completed status in a transaction, then inserts the review. The existing unique constraint on `reviews.job_id` is the final guard against duplicate or concurrent submissions.

**Tech stack:** Go, Fiber, GORM, PostgreSQL, versioned SQL migrations, OpenAPI.

**Scope:** Backend only. No UI, review editing, listing, or Job completion transition.

## Existing behavior and decisions

- `internal/core/domain/review.go` and `reviews` already contain `job_id`, `rating`, optional `comment`, and `created_at`.
- Migration `000002` already makes `reviews.job_id` unique; do not add a duplicate index.
- The current database rating check permits `0`; a new migration must enforce 1–5 for new reviews. Check for existing zero ratings before validating the tighter constraint; do not silently rewrite historical reviews.
- Use the authenticated actor as Creator identity. Never accept a Creator or Marketer ID from the request body.
- `Completed` is the eligibility rule specified by US-033. The endpoint will not require an additional accepted-offer check.
- Proposed response: `201 Created` with `review_id`, `job_id`, `rating`, `comment`, and server-generated `created_at`. Use the existing success/error envelopes.
- Proposed errors: `422` for invalid ID/rating/body, `403` for missing Creator membership or a foreign Job, `404` for a missing Job, and `409` for a non-Completed Job or an existing review.

## Files and responsibilities

- `migrations/000011_review_rating_range.up.sql` and `.down.sql`: tighten/restore the rating check after `dev`'s `000010_job_details` migration; retain the existing unique Job constraint.
- `internal/core/domain/errors.go`: review-specific invalid, incomplete-Job, and duplicate errors.
- `internal/core/ports/review_port.go`: focused submit-review repository interface.
- `internal/services/review_service.go` and `_test.go`: Creator membership, Job ID, rating validation, server timestamp, and repository delegation.
- `internal/repositories/postgres/review_repo.go` and `_integration_test.go`: transactional ownership/status check and insert, including duplicate handling.
- `internal/handlers/http/review_handler.go` and `_test.go`: strict JSON parsing, actor extraction, response DTO, and HTTP error mapping.
- `internal/server/server.go` and server tests: dependency wiring and protected route.
- `docs/openapi.yaml` and `README.md`: document the endpoint and update the statement that reviews are future work.

## Execution steps

### 1. Persistence and schema

- [ ] Inspect existing rows for rating `0` in a disposable migrated test database, if available; report any historical data that blocks a validated 1–5 constraint.
- [ ] Add migration up/down for the rating check. Keep `reviews.job_id` uniqueness intact.
- [ ] Write repository integration tests first for owned Completed Job creation, optional comment and timestamp storage, foreign/missing/incomplete Jobs, and a second submission.
- [ ] Run the targeted tests and confirm they fail for the missing feature.
- [ ] Implement the repository using the existing owned-Job helper within a transaction and map duplicate `job_id` to a domain conflict.
- [ ] Run the targeted tests again and verify the migration directions against a disposable test database when available.

### 2. Service and HTTP contract

- [ ] Write service tests first for Creator membership, invalid Job ID, ratings outside 1–5, optional comment, and repository errors; confirm the expected failures.
- [ ] Implement the service and focused port; run service tests.
- [ ] Write handler tests first for successful `201`, absent comment, malformed/unknown JSON fields, invalid rating, and each authorization/state/conflict error; confirm the expected failures.
- [ ] Implement the handler, register the JWT-protected route, and run handler/server tests.
- [ ] Document request, response, and error codes in OpenAPI and README.

### 3. Verification

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test -count=1 ./...`, `go vet ./...`, and `go build -trimpath ./cmd/api`.
- [ ] Run PostgreSQL integration tests only with an explicit migrated loopback `TEST_DATABASE_URL` whose database name ends in `_test`; report if unavailable.
- [ ] Review the final diff for backend-only scope, migration reversibility, and preservation of unrelated changes.
