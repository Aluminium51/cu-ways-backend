# Production deployment: Railway API and PostgreSQL

Deploy from the backend repository's production branch after its changes have passed CI and been merged. The frontend deployment steps are in the frontend repository's `docs/deployment.md`. This guide prepares Production only; PR previews need separate configuration and data.

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

## 2. Verify

- Check the pre-deploy log for `database migrations are current` and ensure the deployment reaches healthy status.
- Request `/healthz` (process running), `/readyz` (database reachable), and `/docs/openapi.yaml` on the public API domain.
- Verify a protected API route rejects unauthenticated requests with `401`.
- Deploy the frontend with its Production `BACKEND_API_URL` pointing to this public HTTPS domain. The browser uses Next.js BFF routes, so no browser-to-API CORS change is needed for the current application flow.

Do not run `seed-admin` or `seed-mock-users` in Production. Keep database backups and a restore procedure before applying later schema changes. Changes to SQL migrations should remain compatible with the still-running API while Railway activates the new deployment.
