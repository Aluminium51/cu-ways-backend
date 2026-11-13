# CU Ways Backend

## Overview

CU Ways is a platform for survey creators to hire marketers. This Go/Fiber API manages survey metadata, jobs, offers, delivery proof, payment confirmations, and reviews in PostgreSQL.

For the system's roles, job state transitions, and data rules, see [Project context](#project-context).

## Quick start

Requirements

- Go 1.26 or later
- Docker with Docker Compose v2
- GNU Make

Clone the backend repository.

```sh
git clone https://github.com/Aluminium51/cu-ways-backend
cd cu-ways-backend
```

Create a local `.env` from the example if you do not have one. The defaults are for local development. See the available settings in [`.env.example`](.env.example).

**PowerShell**

```powershell
if (-not (Test-Path -LiteralPath .env)) {
    Copy-Item -LiteralPath .env.example -Destination .env
}
```

**Bash**

```bash
if [ ! -e .env ]; then
    cp .env.example .env
fi
```

Start PostgreSQL, apply migrations, and run the API.

```sh
docker compose up -d --wait postgres
make migrate-up
make run
```

The default API address is `http://localhost:8081`. Open [`/healthz`](http://localhost:8081/healthz) to check that the API is running and [`/readyz`](http://localhost:8081/readyz) to check that it can reach PostgreSQL. Both should return HTTP 200.

For optional database browsing, start pgAdmin with `docker compose up -d pgadmin` and open `http://localhost:5050`. Use `postgres` as the database host from pgAdmin and the credentials from your local `.env`. To stop the local Compose services, run `make db-down`.

## API documentation

- [Swagger UI](http://localhost:8081/docs/swagger) and [Scalar](http://localhost:8081/docs) provide interactive API documentation. Both use the [OpenAPI specification](docs/openapi.yaml).
- Follow the [job workflow guide](docs/job-workflow.md) to try the API as a Creator and a Marketer.

API responses use a `status` and `data` or `error` envelope. Use the `X-Request-ID` response header to correlate a request with backend logs.

## Project context

### Backend structure and data ownership

| Area | Responsibility |
| --- | --- |
| `cmd/api`, `internal/server` | API startup, dependency wiring, middleware, and route registration |
| `internal/handlers/http` | HTTP request validation and response mapping |
| `internal/core/domain`, `internal/core/ports` | Business rules, domain types, and interfaces used by services |
| `internal/services` | Use cases, authorization, and validation across domain objects |
| `internal/repositories/postgres` | PostgreSQL queries, row locks, and transaction boundaries |
| `migrations`, `cmd/migrate` | Versioned database schema and migration runner |

Handlers call services, and repositories implement the persistence ports used by services. Make schema changes through versioned SQL migrations. The backend stores survey metadata and job associations. Survey questions and responses are outside its scope. See the [architecture guide](docs/architecture.md).

### Accounts and marketer discovery

- Registration creates a `user` account and its Creator membership in one transaction. A registration with a marketer profile also enables the Marketer workflow. The JWT `role` distinguishes ordinary users and administrators. Creator and Marketer permissions come from memberships and ownership checks. Passwords are stored as Argon2id hashes, and access tokens expire after one hour.
- An optional 10-digit phone number must be unique when provided. Users are soft-deleted and excluded from normal reads. Updates apply only the supplied fields. A client can clear `line_id` with `null` and can replace or clear an existing phone number.
- Marketers manage their own profiles and services. The expertise and campus values come from curated catalogs. Search matches the Marketer's name, bio, and service text, combines selected filters, and excludes archived services. A Marketer appears once even when several services match. Missing prices and ratings sort last. Profile details show completed jobs, qualifying review count, and average rating.

### Job lifecycle and ownership

The normal job status sequence is `Pending` → `Accepted` → `In Progress` → `Completed`. A Creator can also cancel an unpaid job before completion.

| Step | Rules |
| --- | --- |
| Create or edit | A Creator starts with at least one owned survey. Job creation and survey linking commit together. Job details and survey selection can be edited while the job is `Pending`. A survey linked to any job cannot be deleted. |
| Send invitations | Brief changes add a new attachment and preserve history. Invitation batches are atomic. A `Pending` or `Accepted` invitation blocks a second active invitation to the same Marketer for that job. Hiring closes invitations to other Marketers. |
| Respond and submit an offer | Only the recipient can respond to a `Pending` invitation. Accepting the invitation signals interest and enables offer submission while the job is `Pending`. Each invitation can have one active (`Pending` or `Accepted`) offer. An owning Marketer can withdraw a `Pending` offer, then submit a replacement if the job and invitation still qualify. |
| Hire and start | The owning Creator accepts one `Pending` offer. In the same transaction the job becomes `Accepted`, other `Pending` offers become `Rejected`, and other active invitations become `Closed`. The hired Marketer starts the job and moves it to `In Progress`. |
| Cancel | The owning Creator may cancel a `Pending`, `Accepted`, or `In Progress` job without a `Paid` payment. Cancellation closes active invitations, rejects pending offers, and records a server timestamp and any supplied reason. |

The effective delivery deadline is the earliest date set on the job or its linked surveys. If none is set, it is `null`. Offers use the Asia/Bangkok calendar and must be dated from today through that deadline. Invitations show the newest Brief and the sum of the linked surveys' desired responses when available. Creator job summaries count pending offers and exclude withdrawn and rejected offers from that count.

Offer submission, hiring, and cancellation lock the job before applying state changes. A partial unique index protects the one-active-offer rule under concurrent submissions. The accepted-offer guard in migration `000014` checks that the offer belongs to the job and that competing offers were resolved before the job is updated.

### Payment confirmation, proof, and completion

- The owning Creator records the exact accepted price as a decimal string. The supported methods are `PromptPay`, `Bank Transfer`, and `Credit Card`. The service records `Paid` with server-generated timestamps and leaves the job status as it was. The backend records confirmation and simulates payout without moving money through a payment gateway.
- An existing `Pending` or `Paid` payment blocks a second active record. A job lock protects this check under concurrent submissions. Failed and refunded payments remain in history. The Creator can read `/api/v1/jobs/{id}/payments/current` after a reload or a request with an unknown result. Legacy rows may have missing timestamps or the `Other` method.
- The hired Marketer can submit an HTTP or HTTPS Proof URL while the job is `In Progress`. The owning Creator and hired Marketer can read proof history. The database trigger requires a Proof attachment and a non-null `accepted_offer_id` on updates to `Completed`. Attachment rows record the job association, and the API checks uploader permissions against the accepted offer.
- The owning Creator completes an `In Progress` job after proof is saved and a matching `Paid` payment exists. Completion saves the timestamp once, and repeating the request returns the saved result. The settlement read is available to the owning Creator and hired Marketer. For jobs with a `Paid` record, the simulated payout is `Held` while `Accepted` or `In Progress` and `Released` once `Completed`. Cancelled jobs have no payout status.

### Reviews, reporting, and legacy records

- The owning Creator can submit one review after completion, with a rating from 1 to 5 and an optional comment. The current-review endpoint lets the Creator recover the saved review after a reload. Ratings and review counts only include valid reviews from completed jobs. An unrated Marketer has a `null` average.
- The profile owner, Creators, and administrators can use `GET /api/v1/marketers/{id}/reviews` to read verified reviews from completed jobs where the Marketer held the accepted offer. Each review includes the linked job ID and the job owner's name as `reviewer_name`. Reviews are paginated and sorted newest first, with a default of 10 and a maximum of 50 per page. `review_count` and `average_rating` cover the full set. With no qualifying reviews, `items` is empty and `average_rating` is `null`.
- Transaction history lists only completed jobs with a `Paid` record where the signed-in user is the owning Creator or hired Marketer. It lists the accepted price and payment method, sorts by newest completion with unknown times last, and shows `has_review` on the Creator side. Marketer statistics use accepted offers to count completed jobs, omit unrated jobs from the average, and sum recorded paid amounts for completed jobs.
- Migration `000010` adds stored job details. Older jobs may still have empty or null details. Migration `000015` is a compatibility marker for databases that ran the earlier completion migration, `000016` adds cancellation metadata, and `000017` stores the completion instant in `completed_at`. Historical rows without a known completion time retain `null`.

## Development

Run `make verify` before opening a pull request. It runs vet, uncached tests, and the API build. See the [verification guide](docs/verification.md) for PostgreSQL tests, smoke checks, and local seed commands.

See the [architecture guide](docs/architecture.md) for layer responsibilities, migration conventions, and the steps for adding a feature.

Use Conventional Commit subjects for PR titles and commits, for example `docs(api): clarify the job workflow`.

## Deployment

See the [production deployment guide](docs/deployment.md) for the Railway API and PostgreSQL setup and its connection to the Vercel frontend.
