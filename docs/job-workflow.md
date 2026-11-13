# Try the job workflow in Swagger UI

Run the API against a local or staging test database. Open [Swagger UI](http://localhost:8081/docs/swagger) for a local API, or open `/docs/swagger` on your staging API. The steps below use **Try it out** and show paths relative to `/api/v1`.

## Register and authorize

Call `POST /auth/register` twice using the **Creator** and **Marketer** examples. Replace each example's email and phone with unused test values. Save both `data.access_token` values and the Marketer's `data.user.user_id`. You can also get a token with `POST /auth/login` using the account's email and password.

For each protected request, use **Authorize** with the token for the caller shown below. Replace the token when you switch roles. Use IDs from your responses wherever an example contains a placeholder.

## Complete and review a job

| Caller | Request | Input and expected result |
| --- | --- | --- |
| Creator | `POST /surveys` | Use the example and save `data.survey_id`. |
| Creator | `POST /jobs` | Put the survey ID in `survey_ids`. Save `data.job_id` for the job paths below. |
| Creator | `POST /jobs/{id}/briefs` | Use the example Brief URL and the job ID. |
| Creator | `POST /jobs/{id}/requests` | Put the Marketer's user ID in `marketer_ids`. Save `data[0].job_request_id`. |
| Marketer | `POST /me/job-requests/{id}/accept` | Use the job request ID. This request has no body. |
| Marketer | `POST /me/job-requests/{id}/offers` | Use the job request ID and save `data.offer_id` and `data.offered_price`. |
| Creator | `POST /jobs/{id}/offers/{offerId}/accept` | Use the job ID and offer ID. This request has no body. |
| Marketer | `POST /jobs/{id}/start` | Use the job ID. This request has no body. Expect `job_status: In Progress`. |
| Creator | `POST /jobs/{id}/payments` | Use the exact offered price as the string `amount`. The example offer and payment both use `"1500.00"`. Expect `payment_status: Paid` and a `paid_at` timestamp. |
| Marketer | `POST /jobs/{id}/proofs` | Save the example Proof URL against the job ID. |
| Creator | `POST /jobs/{id}/complete` | Use the job ID. This request has no body. Expect `job_status: Completed` and a `completed_at` timestamp. |
| Creator | `POST /jobs/{id}/reviews` and `GET /jobs/{id}/reviews/current` | Submit the example rating, then check that the saved review is returned. |
| Creator | `GET /me/transactions?role=creator` | Check that `data.items` includes the job with the accepted price and `has_review: true`. |

The sample delivery date is for the example job with no deadline. For a job with a deadline, choose a date from today through the effective deadline in the Asia/Bangkok calendar. The effective deadline is the earliest available date across the job and its linked surveys.

The payment records the Creator's confirmation. Payout is simulated and no funds move through a payment gateway.

## Run the automated workflow

From the backend repo, set `TEST_DATABASE_URL` to a disposable PostgreSQL database on loopback with a name ending in `_test`. The database account must be able to create schemas. The test applies migrations to a temporary schema, exercises the served OpenAPI examples through the API, and removes the schema when it finishes.

```sh
go test -count=1 -run '^TestSwaggerWorkflow$' -v ./internal/server
```

See the [PostgreSQL integration guide](verification.md#postgresql-integration) for the URL format and database setup.
