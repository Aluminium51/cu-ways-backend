# CU Ways Backend

Go/Fiber backend for CU Ways. The project uses a layered/hexagonal structure so HTTP handlers, business logic, database access, and infrastructure can evolve independently.

The backend provides configuration, PostgreSQL connectivity, migrations, health checks, JWT verification middleware, structured logging, Docker development services, and User CRUD endpoints. Survey, job, login, and refresh-token workflows remain future work.

For the full design rules, see [docs/architecture.md](docs/architecture.md).

## Requirements

- Go 1.26+
- Docker Desktop with Docker Compose
- GNU Make

All commands below are run from this directory:

```powershell
cd D:\test-fullstack\cu-way\backend
```

## Quick start

1. Create `.env` if it does not exist:

   ```powershell
   Copy-Item .env.example .env
   ```

   Keep an existing `.env`; it contains local credentials and is ignored by Git.

2. Start PostgreSQL and pgAdmin:

   ```powershell
   docker compose up -d postgres pgadmin
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
├── cmd/api/                  # Application entry point
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

## User API

User CRUD endpoints are available under `/api/v1/users`:

| Method | Path | Access |
| --- | --- | --- |
| `POST` | `/api/v1/users` | Public |
| `GET` | `/api/v1/users/:id` | JWT owner or admin |
| `GET` | `/api/v1/users` | JWT admin only |
| `PUT` | `/api/v1/users/:id` | JWT owner or admin |
| `DELETE` | `/api/v1/users/:id` | JWT owner or admin |

List requests support `page` and `page_size` query parameters. Pages start at `1`, the default page size is `20`, and the maximum page size is `100`. Deleted users are soft-deleted and excluded from normal reads and lists.

Example create request:

```powershell
curl.exe -X POST http://localhost:8081/api/v1/users `
  -H "Content-Type: application/json" `
  -d '{"name":"Jane Doe","email":"jane@example.com","phone":"0812345678","line_id":"jane.line"}'
```

Protected requests require a JWT whose `sub` claim is the numeric user ID. The `role` claim must be `admin` for administrator access.

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

The current migrations include a foundation baseline, the domain schema, and the user soft-delete column. Migrations are the database source of truth; do not use GORM `AutoMigrate` for this project.
