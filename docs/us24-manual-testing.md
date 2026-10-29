# US24: offer withdrawal verification

## Verified behavior

An authenticated Marketer can withdraw only their own `Pending` offer using
`POST /api/v1/me/offers/{offerId}/withdraw`, with no request body.
The response is `200` with the normal success envelope and the updated offer:

```json
{
  "status": "success",
  "data": {
    "offer_id": 1,
    "job_id": 1,
    "job_request_id": 1,
    "marketer_id": 2,
    "offered_price": "100.00",
    "delivery_date": "2026-10-23",
    "message": "Original terms",
    "offer_status": "Withdrawn",
    "created_at": "2026-10-16T03:09:58.656982Z"
  }
}
```

The values above are illustrative. Only `offer_status` changes in the stored
record. The offer is retained as history; its job remains `Pending`, its
invitation remains `Accepted`, and competing offers are unaffected. The Marketer
may submit a replacement through the existing submission endpoint. It receives a
new offer ID and `Pending` status. Only one `Pending` or `Accepted` offer may exist
per invitation.

Accepting the invitation is not hiring. The Creator hires the Marketer by
accepting an offer, which moves the job to `Accepted`.

## Start the backend

From the backend directory, use your existing local `.env` configuration:

```powershell
docker compose up -d postgres
make migrate-up
go run ./cmd/api
```

Leave the API running. The default base URL is `http://localhost:8081`.
Use Postman, another API client, or the Scalar reference at
`http://localhost:8081/docs`. The frontend does not need to implement US24 for
these checks. Each protected request needs `Authorization: Bearer <access_token>`.
JSON requests need `Content-Type: application/json`.

Use local test accounts and data. The manual workflow creates real records in
whichever local database your API is configured to use.

## Prepare a pending offer

1. Create or log in as three users: a Creator without Marketer membership,
   Marketer A, and Marketer B. To register a Marketer, include `marketer_profile`:

   ```http
   POST /api/v1/auth/register
   ```

   ```json
   {
     "name": "US24 Marketer A",
     "email": "us24-marketer-a@example.test",
     "password": "local-test-password-123",
     "phone": "0900000024",
     "marketer_profile": {
       "bio": "Student survey distribution",
       "experience_years": 1,
       "availability_text": "Available for testing"
     }
   }
   ```

   Use distinct emails and unique 10-digit phone numbers for each new account.
   Omit `marketer_profile` when registering the Creator. Registration returns
   `data.access_token` and `data.user.user_id`. Existing users can obtain a token
   from `POST /api/v1/auth/login` with `email` and `password`.

2. As the Creator, create a survey using `POST /api/v1/surveys`:

   ```json
   { "title": "US24 test", "survey_link": "https://example.test/survey" }
   ```

3. As the Creator, create a job using `POST /api/v1/jobs` with
   `{"survey_ids":[1]}`. Replace `1` with the survey ID returned above.
   Expect `201`, job status `Pending`, and `accepted_offer_id: null`.

4. As the Creator, invite both Marketers using
   `POST /api/v1/jobs/{jobId}/requests` with `{"marketer_ids":[2,3]}`.
   Replace the example IDs with their actual user IDs. Expect `201` and a list
   containing a separate `job_request_id` for each Marketer.

5. Each Marketer accepts their own invitation with
   `POST /api/v1/me/job-requests/{jobRequestId}/accept`, with no body.
   Expect `200` and `request_status: "Accepted"`.

6. Each Marketer submits an offer using
   `POST /api/v1/me/job-requests/{jobRequestId}/offers`:

   ```json
   {
     "offered_price": "100.00",
     "delivery_date": "2026-10-23",
     "message": "Original terms"
   }
   ```

   **Replace the sample delivery date with today or a future date in Bangkok.**
   If the survey has a deadline, delivery must not be after it. Expect `201` and
   `offer_status: "Pending"`. Save each returned `offer_id`.

## Exercise withdrawal and resubmission

Use Marketer A's token unless the table says otherwise. `{offerId}` below means
the offer ID, not the invitation ID or job ID.

| Action | Expected result |
| --- | --- |
| `GET /api/v1/me/job-requests/{jobRequestId}/offers` | Save the persisted original offer for comparison. |
| `POST /api/v1/me/offers/{offerId}/withdraw`, no body | `200`; status becomes `Withdrawn`. Original terms and stored creation time are preserved. |
| Repeat the withdrawal | `409 offer_not_pending`; no additional changes. |
| Read the invitation with `GET /api/v1/me/job-requests` | Invitation still `Accepted`; nested job still `Pending`. |
| As Creator, `POST /api/v1/jobs/{jobId}/offers/{offerId}/accept` on the withdrawn offer | `409 offer_not_pending`; job remains unassigned. |
| Submit another offer through the same invitation, e.g. price `95.00` | `201`; a new ID and `Pending` status. |
| Submit again before withdrawing that replacement | `409 offer_already_exists`. |
| Read the invitation's offers again | Replacement first, withdrawn original retained as history. |
| As Creator, `GET /api/v1/jobs/{jobId}/offers` | Both original and replacement remain visible, with their respective statuses. |
| As Creator, accept the replacement | `200`; job becomes `Accepted`, replacement becomes `Accepted`, competing Pending offers become `Rejected`. |
| Try to withdraw the accepted replacement | `409 offer_not_pending`. |
| As Marketer B, try to withdraw B's rejected offer | `409 offer_not_pending`. |
| Try to submit another offer after hiring | `409 job_not_pending`. |

Compare creation times against the persisted GET response. PostgreSQL stores
microsecond precision, while an initial creation response can contain additional
in-memory nanosecond digits.

For a quick PowerShell withdrawal, replace the example IDs and token:

```powershell
$base = 'http://localhost:8081/api/v1'
$marketerToken = '<Marketer A access_token>'
$offerId = 1
$requestId = 1
$headers = @{ Authorization = "Bearer $marketerToken" }
Invoke-RestMethod -Method Post -Uri "$base/me/offers/$offerId/withdraw" -Headers $headers
Invoke-RestMethod -Uri "$base/me/job-requests/$requestId/offers" -Headers $headers
```

## Check rejected requests

Run the foreign-owner check on a fresh Pending offer, before withdrawing it.
After every rejected action, read the offer with its owner's token to confirm
that the status and terms did not change.

| Request | Expected HTTP status and error code |
| --- | --- |
| No bearer token, or invalid token | `401 unauthorized` |
| Creator without Marketer membership | `403 marketer_profile_required` |
| Marketer B attempts to withdraw A's offer | `404 offer_not_found` |
| Nonexistent positive offer ID | `404 offer_not_found` |
| ID `0`, `-1`, `abc`, or `2147483648` | `422 validation_error` |
| Owner withdraws an Accepted, Rejected, or Withdrawn offer | `409 offer_not_pending` |

## Concurrent actions

Use a fresh Pending offer for each trial. Send Creator acceptance and Marketer
withdrawal concurrently from separate API clients. Exactly one succeeds with
`200`; the other returns `409 offer_not_pending`. If withdrawal wins, the offer
is `Withdrawn` and the job remains `Pending`. If acceptance wins, both offer and
job are `Accepted`. Neither request should deadlock or return `500`.

Sending two withdrawals concurrently must produce one `200` and one `409`,
with a single retained `Withdrawn` record.

## Verification on 16 October 2026

- Windows: full uncached suite with PostgreSQL integration and coverage passed.
- Linux: full uncached suite with PostgreSQL integration, race detection, and
  atomic coverage passed. Each run reported 289 passing tests/subtests across
  16 packages, zero failed tests, and zero skipped tests.
- `golangci-lint` v2.12.2, `go vet`, all-command build, and API build passed.
- All nine migrations applied to fresh PostgreSQL 18.6 databases.
- Real HTTP verification passed 95 checks, including ten simultaneous
  acceptance/withdrawal trials. Withdrawal won nine trials and acceptance won
  one; exactly one transition succeeded in each trial. Concurrent duplicate
  withdrawal also returned one success and one conflict.
- No US24 functional defect was found. Windows' local MinGW compiler could not
  run race instrumentation, so race detection was verified in Linux instead.

One existing shared diagnostics bug was observed: `RequestLogger` in
`internal/middleware/request.go` reads the response status before the central
error handler processes a returned error. A rejected acceptance/withdrawal can
therefore produce an `http request` log with `status=200` followed by a
`request rejected` log with `status=409` for the same request ID. The actual
HTTP response is `409`, and the database state remains correct. This middleware
issue was left unchanged on the US24 branch; it needs a separate regression
test and fix for logging the final response status.

Ignored local evidence files are `coverage/us24-tests.jsonl`,
`coverage/us24-race-tests.jsonl`, their coverage profiles, and
`coverage/us24-http-results.json`. They contain no access tokens.
