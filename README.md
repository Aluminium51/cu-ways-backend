# CU Ways Backend

Go/Fiber backend foundation for CU Ways. It currently provides configuration, PostgreSQL connectivity, migrations, health checks, JWT verification middleware, structured logging, and Docker development services.

User, survey, job, login, and refresh-token features are not implemented yet.

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

## API checks

```powershell
curl.exe -i http://localhost:8081/healthz
curl.exe -i http://localhost:8081/readyz
```

- `/healthz` confirms that the API process is running.
- `/readyz` confirms that PostgreSQL is reachable.
- Responses use a standard `status` and `data`/`error` envelope.
- Requests receive an `X-Request-ID` response header.

OpenAPI documentation is available at [docs/openapi.yaml](docs/openapi.yaml).

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

## Project layout

```text
cmd/api              Application entrypoint
internal/config      Environment configuration
internal/handlers    HTTP handlers
internal/middleware  Recovery, logging, request IDs, JWT verification
internal/server      Fiber app and route setup
internal/services    Business/service boundaries
internal/core/ports  Application interfaces
pkg/database         GORM PostgreSQL connection
pkg/response          Standard API responses and errors
pkg/utils             JWT and validation helpers
migrations            SQL migration files
docs                  OpenAPI documentation
```

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
```

The current migration is only a foundation baseline; it does not create domain tables.
