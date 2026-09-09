# CU Ways Backend

Go/Fiber backend for CU Ways. The project uses a layered/hexagonal structure so HTTP handlers, business logic, database access, and infrastructure can evolve independently.

The backend provides configuration, PostgreSQL connectivity, migrations, health checks, JWT authentication, structured logging, Docker development services, user CRUD, marketer profiles and search, services, and survey metadata management. Job, offer, payment, review, and refresh-token workflows remain future work.

For the full design rules, see [docs/architecture.md](docs/architecture.md).

## Requirements

- Go 1.26+
- Docker Desktop with Docker Compose
- GNU Make

First, Clone the repository

```powershell
git clone https://github.com/cu-ways/cu-ways-backend.git
cd cu-ways-backend
```

## Quick start

1. Create `.env` if it does not exist:

   ```powershell
   Copy-Item .env.example .env
   ```

    for Mac/Linux:

    ```bash
    cp .env.example .env
    ```

   Keep an existing `.env`; it contains local credentials and is ignored by Git.

2. Start PostgreSQL and pgAdmin:

   ```powershell
   docker compose up -d --build
   docker compose ps
   ```

3. Apply migrations:

   ```powershell
   make migrate-up
   ```

4. Start the API:

   ```powershell
   make run
   ```

The API runs on `http://localhost:8081` by default.

## Project structure

```text
cu-ways-backend/
├── cmd/
│   ├── api/                  # Application entry point
│   └── seed-admin/           # Explicit development/test admin seeder
├── docs/                     # OpenAPI and architecture documentation
├── internal/                 # Private application code
│   ├── config/               # Environment and configuration loader
│   ├── core/
│   │   ├── domain/           # Entities and persistence/domain models
│   │   └── ports/            # Interfaces for application boundaries
│   ├── handlers/http/        # Fiber HTTP transport layer (package httpapi)
│   ├── middleware/           # Recovery, request ID, logging, and JWT middleware
│   ├── platform/             # Database, logging, response, and utility adapters
│   ├── repositories/postgres/ # PostgreSQL repository implementations
│   ├── server/               # Fiber app composition and route registration
│   └── services/             # Business logic and use-case orchestration
├── migrations/               # PostgreSQL .up.sql and .down.sql files
├── Dockerfile
├── docker-compose.yml
├── Makefile                  # Developer command shortcuts
└── go.mod
```

## Implementing a new feature

Follow these six steps when adding a feature such as users, surveys, or jobs:

1. **Define the domain** — Add or update entities and business value types in `internal/core/domain`. Keep them independent from Fiber and database connection details.
2. **Define the port** — Add a focused interface in `internal/core/ports` describing what the service needs from persistence or another external system.
3. **Implement the service** — Add the use-case and business rules in `internal/services`. Services depend on ports, not concrete PostgreSQL repositories.
4. **Implement persistence** — Add the PostgreSQL/GORM implementation under `internal/repositories/postgres`. Keep SQL queries, preload choices, and database error mapping here.
5. **Expose HTTP behavior** — Add request/response DTOs and Fiber handlers under `internal/handlers/http`, then register routes through `internal/server`.
6. **Verify the feature** — Add or update migrations, tests, OpenAPI documentation, and run the checks below before committing.

Do not call a repository directly from a handler, put business rules in a handler, or make domain packages import Fiber or GORM connection setup. See [docs/architecture.md](docs/architecture.md) for the complete rules.

## API checks

```powershell
curl.exe -i http://localhost:8081/healthz
curl.exe -i http://localhost:8081/readyz
```

- `/healthz` confirms that the API process is running.
- `/readyz` confirms that PostgreSQL is reachable.
- Responses use a standard `status` and `data`/`error` envelope.
- Requests receive an `X-Request-ID` response header.

API documentation is available at:

- Interactive Scalar reference: [http://localhost:8081/docs](http://localhost:8081/docs)
- Raw OpenAPI YAML: [http://localhost:8081/docs/openapi.yaml](http://localhost:8081/docs/openapi.yaml)

The Scalar interface loads from a CDN when the page opens, so the browser needs internet access. The raw specification is served directly from `docs/openapi.yaml`.

## Authentication API

Register and login are public endpoints:

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/auth/register` | Create an account and receive an access token |
| `POST` | `/api/v1/auth/login` | Verify email/password and receive an access token |

Register creates accounts with the `user` role and provisions the user's Creator membership atomically. Passwords are stored as Argon2id hashes and never returned in API responses. Access tokens use `HS256` and expire after one hour.

Register:

```powershell
curl.exe -X POST http://localhost:8081/api/v1/auth/register `
  -H "Content-Type: application/json" `
  -d '{"name":"Jane Doe","email":"jane@example.com","password":"correct horse battery staple"}'
```

Login:

```powershell
curl.exe -X POST http://localhost:8081/api/v1/auth/login `
  -H "Content-Type: application/json" `
  -d '{"email":"jane@example.com","password":"correct horse battery staple"}'
```

Use the returned `data.access_token` on protected endpoints:

```powershell
$token = "<access_token>"
curl.exe http://localhost:8081/api/v1/users/1 `
  -H "Authorization: Bearer $token"
```

To create a local administrator account, configure the seed credentials in `.env` or set them for one PowerShell session:

```powershell
$env:SEED_ADMIN_NAME = "CU Ways Admin"
$env:SEED_ADMIN_EMAIL = "admin@example.com"
$env:SEED_ADMIN_PASSWORD = "local-admin-password-123"

make seed-admin
```

The seed command is allowed only in development/test environments. It creates the account when missing, or promotes an existing active account without changing its password. It never restores a soft-deleted account or overwrites credentials.

Account creation is handled only by `/api/v1/auth/register`, which stores a password and provisions Creator membership. The `/api/v1/users` resource is for reading and updating users after registration.

## User API

User CRUD endpoints are available under `/api/v1/users`:

| Method | Path | Access |
| --- | --- | --- |
| `GET` | `/api/v1/users/:id` | JWT owner or admin |
| `GET` | `/api/v1/users` | JWT admin only |
| `PUT` | `/api/v1/users/:id` | JWT owner or admin |
| `DELETE` | `/api/v1/users/:id` | JWT owner or admin |

List requests support `page` and `page_size` query parameters. Pages start at `1`, the default page size is `20`, and the maximum page size is `100`. Deleted users are soft-deleted and excluded from normal reads and lists.

Protected requests require a JWT whose `sub` claim is the numeric user ID. The `role` claim must be `admin` for administrator access.

## Marketer and Survey APIs

Marketer profile fields are saved through `PATCH /api/v1/me/marketer-profile`. The core fields `bio`, `experience_years`, `availability_status`, and `availability_text` are required and cannot be empty. Expertise and campus values use the curated catalog; their arrays may be empty.

The initial expertise slugs are `survey-distribution`, `participant-recruitment`, `data-collection`, `quantitative-analysis`, `qualitative-analysis`, and `report-preparation`. The initial campus slugs are `cu-main-campus`, `cu-health-sciences-campus`, `off-campus`, and `online-remote`. Availability is `available`, `limited`, or `unavailable`.

Phone numbers are optional, stored as trimmed digits, and must be unique whenever present. A duplicate phone returns `409 phone_already_exists`.

```powershell
curl.exe -X PATCH http://localhost:8081/api/v1/me/marketer-profile `
  -H "Authorization: Bearer $token" `
  -H "Content-Type: application/json" `
  -d '{"bio":"Survey research specialist","experience_years":4,"availability_status":"available","availability_text":"Available on weekdays","expertise":["data-collection","report-preparation"],"campuses":["cu-main-campus"]}'
```

Marketers manage their services under `/api/v1/me/services`. Creators and administrators can search marketers with `GET /api/v1/marketers`. Combine `min_price`, `max_price`, repeated `expertise` and `campus`, `min_rating`, `min_experience_years`, and `availability_status` filters. All selected filters must match. Ratings include only valid 1–5 reviews from completed jobs; the default order is lowest matching service price, then average rating.

```powershell
curl.exe "http://localhost:8081/api/v1/marketers?min_price=500&max_price=3000&expertise=data-collection&campus=cu-main-campus&sort=price_asc" `
  -H "Authorization: Bearer $token"
```

Create surveys with `POST /api/v1/surveys`. Registration already provisions the Creator membership; survey creation also uses an idempotent membership insert so existing and legacy accounts can create surveys safely. Survey owners can read, edit, and delete their surveys with `/api/v1/surveys/:id`. A survey referenced by any `is_used_in` row cannot be deleted, even when the related job is no longer active.

## Database tools

PostgreSQL:

```text
Host:     localhost
Port:     5432
Database: cuway_database
User:     myuser
Password: mypassword
```

pgAdmin is available at [http://localhost:5050](http://localhost:5050).

```text
Email:    admin@gmail.com
Password: weakpassword
```

When registering PostgreSQL in pgAdmin, use `postgres` as the host because pgAdmin runs inside Docker Compose. Use port `5432`, database `cuway_database`, user `myuser`, and password `mypassword`.

Change the example passwords before using this setup outside local development.

## Useful commands

| Command | Purpose |
| --- | --- |
| `make run` | Start the API |
| `make build` | Build the API binary |
| `make test` | Run all tests |
| `make vet` | Run `go vet` |
| `make fmt` | Format Go code |
| `make db-up` | Start PostgreSQL |
| `make db-down` | Stop Compose services |
| `make migrate-up` | Apply migrations |
| `make migrate-down` | Roll back one migration |
| `make migrate-version` | Show the current migration version |
| `make seed-admin` | Create or promote the development admin account |

## Configuration

`.env.example` contains the supported settings. Important values include:

```env
PORT=8081
DATABASE_URL=postgresql://myuser:mypassword@localhost:5432/cuway_database?sslmode=disable
SECRET_KEY=replace_with_a_32_character_secret_key
```

Environment variables override values from `.env`. The local PostgreSQL URL should include `sslmode=disable` because the development container does not enable TLS.

## Development checks

Before submitting changes, run:

```powershell
go test ./...
go vet ./...
go build -trimpath ./cmd/api
go list ./...
```

The current migrations include a foundation baseline, the domain schema, the user soft-delete column, authentication columns, and normalized marketer profile/search tables. Migrations are the database source of truth; do not use GORM `AutoMigrate` for this project.
