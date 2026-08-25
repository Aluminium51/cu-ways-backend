# CU Ways Backend Architecture

## Purpose

This document defines the boundaries and dependency rules for the CU Ways backend. It is intended for maintainers and developers adding domain features such as users, surveys, jobs, offers, payments, and reviews.

The architecture is layered with hexagonal boundaries:

```text
HTTP request
    │
    ▼
middleware → handlers/http → services → core/ports ← repositories/postgres
                                      │                    │
                                      ▼                    ▼
                                  core/domain       platform/database

cmd/api and internal/server compose the concrete application at startup.
```

The dependency direction points inward. Business rules must not depend on Fiber, PostgreSQL, or GORM implementation details.

## Layer responsibilities

### `cmd/api`

Process entry point and lifecycle owner.

- Loads configuration.
- Creates logging and database dependencies.
- Builds the Fiber application through `internal/server`.
- Starts the HTTP listener.
- Handles `SIGINT`/`SIGTERM` and graceful shutdown.

`cmd/api` is the only place that should own the complete application startup sequence.

### `internal/config`

Loads and validates environment configuration.

- Reads `.env` for local development.
- Allows environment variables to override file values.
- Validates database URLs, ports, secrets, and pool settings.
- Does not contain business configuration or database queries.

### `internal/core/domain`

Contains entities and domain-level types shared by application use cases.

Current models include:

- `User`, `Creator`, and `Marketer`
- `Service` and `Survey`
- `Job` and `JobSurvey`
- `Offer`, `Attachment`, `Payment`, and `Review`
- Status and type constants used by the database constraints

Domain code must remain independent from Fiber, HTTP request types, and database connection setup. GORM mapping tags are currently retained on persistence models because the project uses these structs for database mapping; business rules should still be kept in services rather than handlers or database adapters.

Request and response DTOs should be defined near the HTTP feature handler, not added to the domain entities solely for transport formatting.

### `internal/core/ports`

Defines interfaces at application boundaries.

Current ports:

- `ReadinessChecker` for dependency health checks.
- `TokenVerifier` for JWT verification.

When a feature is introduced, add a focused port describing the capability a service needs. The interface belongs to the consumer side, normally the service/application layer. PostgreSQL repositories implement those interfaces.

Do not create speculative `user_port.go`, `survey_port.go`, or `job_port.go` files before their use cases exist.

### `internal/services`

Contains business logic and use-case orchestration.

Current service:

- `HealthService`, which checks PostgreSQL readiness with a timeout.

Feature services should:

- Accept ports through constructors.
- Coordinate domain rules and transactions.
- Return application-level results or errors.
- Avoid importing Fiber request/response types.
- Avoid importing concrete PostgreSQL repository packages.

Services are the correct place for state transitions such as accepting an offer, completing a job, or validating a payment transition.

### `internal/handlers/http`

Fiber HTTP transport layer, using package name `httpapi`.

Handlers are responsible for:

- Reading path, query, header, and body input.
- Decoding and validating request DTOs.
- Calling the appropriate service.
- Translating service results to HTTP status codes and response envelopes.
- Avoiding direct database or repository calls.

Handlers should not contain multi-step business rules. A handler should remain thin enough that its behavior can be tested with Fiber requests and mocked service dependencies.

### `internal/repositories/postgres`

PostgreSQL persistence implementations.

This package currently contains only a boundary marker because feature repositories are not implemented yet. When added, repositories will:

- Implement ports from `internal/core/ports`.
- Use the shared GORM database connection from `internal/platform/database`.
- Query and persist domain models.
- Handle preloading, pagination, uniqueness, and database-specific errors.
- Avoid deciding business state transitions.

Repository code must not be called directly by HTTP handlers.

### `internal/platform`

Infrastructure adapters and reusable technical utilities:

- `platform/database` — GORM PostgreSQL connection, pool settings, ping, and close.
- `platform/logging` — Zerolog configuration.
- `platform/response` — API envelopes, application errors, and Fiber error translation.
- `platform/utils` — JWT verification and shared validation helpers.

Platform packages may depend on external libraries and configuration, but should not depend on feature services or HTTP handlers.

### `internal/middleware`

Cross-cutting Fiber middleware:

- Panic recovery.
- Request IDs.
- Request logging.
- JWT authentication middleware.

Middleware may translate authentication failures into the standard response error format, but authorization and business decisions belong in services.

### `internal/server`

Application composition and route registration.

The server package:

- Creates the Fiber app.
- Installs global middleware.
- Constructs or receives handler dependencies.
- Registers routes such as `/healthz` and `/readyz`.

It is a composition boundary, not a business-logic package.

## Dependency rules

The following rules are mandatory:

1. `core/domain` must not import Fiber, GORM connection setup, or HTTP packages.
2. `core/ports` must contain abstractions, not PostgreSQL implementations.
3. `services` depend on ports and domain types, never concrete repositories.
4. `handlers/http` depend on services and response helpers, never database connections.
5. `repositories/postgres` implements ports and may use GORM and platform database access.
6. `platform` must not import handlers or feature services.
7. `cmd/api` and `server` are allowed to wire concrete implementations together.
8. No package may introduce an import cycle to bypass these boundaries.
9. New feature code belongs under `internal`; `pkg` is not used for backend-only implementation details.

## Request flow

A normal feature request follows this path:

```text
Client
  → Fiber middleware
  → httpapi handler
  → service/use case
  → port interface
  → PostgreSQL repository
  → GORM/database
```

The response travels back through the same layers. Database errors should be logged with context and translated into safe application errors before reaching the client. Internal SQL details and secrets must never be returned in an API response.

## Adding a new feature

Use this sequence for a new feature:

1. Define or update the domain entity and constrained values.
2. Add the database migration in `migrations` before writing repository queries.
3. Define the smallest required port in `internal/core/ports`.
4. Implement the business use case in `internal/services`.
5. Implement the PostgreSQL repository in `internal/repositories/postgres`.
6. Add request/response DTOs and handlers in `internal/handlers/http`.
7. Register routes and inject dependencies through `internal/server`.
8. Document the endpoint in `docs/openapi.yaml`.
9. Add tests at the domain, service, repository, and handler levels as appropriate.

Do not add a repository, service, or port only because a directory tree anticipates it. Add the boundary when the feature has a real use case and a testable contract.

## Persistence and migrations

SQL migrations are the source of truth for PostgreSQL schema changes.

- `000001_foundation` establishes the migration workflow.
- `000002_domain_schema` creates the current domain tables, constraints, indexes, and foreign keys.
- Use `.up.sql` for forward changes and `.down.sql` for rollback behavior.
- Run `make migrate-up` after starting PostgreSQL.
- Use `make migrate-version` to inspect the applied version.
- Do not call GORM `AutoMigrate` from application startup.

GORM models must match the existing SQL column names and nullability. Schema changes require a migration and model review together.

## Errors and responses

Successful API responses use:

```json
{
  "status": "success",
  "data": {}
}
```

Errors use:

```json
{
  "status": "error",
  "error": {
    "code": "database_unavailable",
    "message": "service is not ready"
  }
}
```

Handlers should return application errors or service results to the centralized Fiber error handler. The error handler owns HTTP status mapping, safe client messages, and structured logging.

## Testing strategy

Use the narrowest test level that gives confidence:

- **Domain tests** — Validate constants, model mappings, and domain invariants.
- **Service tests** — Use fake ports to test business rules without PostgreSQL.
- **Repository tests** — Use the Docker PostgreSQL service for query and mapping behavior.
- **Handler tests** — Use Fiber `app.Test` with mocked service dependencies.
- **Integration checks** — Run migrations and verify readiness against PostgreSQL.

Before committing, run:

```powershell
go test ./...
go vet ./...
go build -trimpath ./cmd/api
go list ./...
```

## Anti-patterns to avoid

- Calling GORM or `*sql.DB` directly from an HTTP handler.
- Putting business state transitions in Fiber handlers.
- Importing Fiber into domain or service packages.
- Importing concrete PostgreSQL repositories into services.
- Using global mutable database or service variables.
- Running `AutoMigrate` on application startup.
- Returning raw database errors to clients.
- Adding broad `ports.go` interfaces instead of focused feature ports.
- Adding empty feature packages without a use case or contract.
- Reusing persistence models as public API responses when their fields or relationships do not match the API contract.

## Current status

The architecture foundation is complete, but feature repositories, services, handlers, authentication endpoints, and domain workflows are still future work. The current live endpoints are `/healthz` and `/readyz`.
