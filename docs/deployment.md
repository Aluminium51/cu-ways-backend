# Production deployment: Railway API and PostgreSQL

Deploy from each repository's production branch after its changes have passed CI and been merged. Use a separate environment and database for staging and previews.

## 1. Prepare Railway

1. Create a Railway project and add a PostgreSQL service. Keep the database private and enable backups appropriate to the data you will retain.
2. Add a service from this backend GitHub repository. Select the production branch (normally `main`). Railway detects the root `Dockerfile`.
3. In the API service Variables, set:

   | Variable | Value |
   | --- | --- |
   | `APP_ENV` | `production` |
   | `DATABASE_URL` | Reference the database service's `DATABASE_URL`, for example `${{Postgres.DATABASE_URL}}` if its service name is `Postgres` |
   | `SECRET_KEY` | A unique, randomly generated secret of at least 32 characters |

   Railway supplies `PORT`; the API listens on that port. Set database pool/timeouts only when the defaults in `.env.example` are unsuitable. Never copy local `.env` credentials, seed passwords, or `TEST_DATABASE_URL` to Production.

4. `railway.json` runs `/migrate` before the new API deployment and checks `/readyz` before routing traffic. A failed migration stops deployment. The API does not migrate or seed the database on startup. The runner is safe to execute again when the schema is already current.
5. Generate a public HTTPS domain for the API and record it for the frontend's server-only `BACKEND_API_URL`.

The API image is distroless. Its default command is `/api`; `/migrate` is a separate static binary. The image also contains `docs/` and `migrations/`. Neither binary needs a shell. Do not set a custom start command unless the default is intentionally replaced.

## 2. Connect Vercel and verify

In the frontend Vercel project, set the server-only `BACKEND_API_URL` to the Railway API's HTTPS origin. Set `FRONTEND_ORIGIN` to the canonical HTTPS origin of the deployed frontend, including the scheme and omitting the trailing slash. Keep `MARKETER_DEMO_ENABLED=false`.

- Check the pre-deploy log for `database migrations are current` and ensure the deployment reaches healthy status.
- Request `/healthz` (process running), `/readyz` (database reachable), and `/docs/openapi.yaml` on the public API domain.
- Verify a protected API route rejects unauthenticated requests with `401`.
- Register and sign in through the deployed frontend. Next.js sends API requests on the browser's behalf. The API applies its login and registration quota to the connection IP it sees, so verify authentication through the actual Vercel and Railway ingress before enabling public sign-up.

## 3. Provision the first administrator

1. Have the designated operator register through the deployed website. Confirm their email directly with them, then look up the matching user ID in the private PostgreSQL database.
2. In a PostgreSQL session with write access, run the following transaction using the verified ID and email in place of the examples.

   ```sql
   BEGIN;
   UPDATE users
   SET role = 'admin'
   WHERE user_id = 123
     AND email = 'operator@example.invalid'
     AND role = 'user'
     AND deleted_at IS NULL
     AND password_hash IS NOT NULL
   RETURNING user_id, email, role;
   ```

   Run `COMMIT;` after verifying that exactly one row identifies the intended operator. Otherwise run `ROLLBACK;`.

3. Have the operator sign out and sign back in. Verify that they can open the Users, Jobs and payments, and Flagged marketers pages from the admin workspace.

Keep `seed-admin` and `seed-mock-users` for local environments. Keep database backups and a restore procedure before applying later schema changes. Changes to SQL migrations should remain compatible with the still-running API while Railway activates the new deployment.
