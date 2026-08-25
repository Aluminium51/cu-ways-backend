# CU Ways Backend Architecture

## Purpose

This document defines the boundaries and dependency rules for the CU Ways backend. It is intended for maintainers and developers adding domain features such as users, surveys, jobs, offers, payments, and reviews.

## Project structure

The repository combines the standard Go project layout with Hexagonal Architecture (Ports and Adapters). The tree below is representative; feature-specific files should be added only when a real use case requires them.

```text
cu-ways-backend/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point and lifecycle owner
├── docs/                           # OpenAPI contract and architecture documentation
├── internal/
│   ├── config/                     # Environment loading and configuration validation
│   ├── core/
│   │   ├── domain/                 # Entities and domain types; standard library only, zero external imports
│   │   └── ports/                  # Consumer-side interfaces required by use cases
│   ├── handlers/
│   │   └── http/                   # Fiber HTTP adapters; package httpapi
│   ├── middleware/                 # Request ID, logging, recovery, and JWT middleware
│   ├── platform/                   # Technical adapters: database, logging, responses, and utilities
│   ├── repositories/
│   │   └── postgres/               # PostgreSQL/GORM implementations of ports
│   ├── server/                     # Fiber composition, dependency injection, and route registration
│   └── services/                   # Business logic, use cases, and state transitions
├── migrations/                     # Raw SQL migrations: .up.sql and .down.sql
├── Dockerfile                      # Production container image
├── docker-compose.yml              # Local PostgreSQL infrastructure
├── Makefile                        # Common development and migration commands
└── go.mod                          # Module definition and dependency versions
```

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

Domain code must remain independent from Fiber, GORM, HTTP request types, and database connection setup. Keep domain entities free of external package imports; persistence-specific mapping belongs in repository adapters or dedicated persistence models. Business rules should still be kept in services rather than handlers or database adapters.

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
10. Request-scoped work must propagate `context.Context` from the HTTP adapter through services, ports, and repositories.
11. Services may coordinate transactions through a port, but must never receive or import `*gorm.DB` or `*sql.Tx`.
12. HTTP handlers validate transport format and syntax; services validate business meaning and invariants.
13. Repositories enforce persistence concerns and translate database constraint errors; they do not own business decisions.

## Context propagation

Context is part of every application boundary contract. Handlers obtain the request context from Fiber with `c.UserContext()` and pass it unchanged to service methods. Services pass the context to ports, and repositories use it for all database operations.

```go
func (h *UserHandler) Get(c *fiber.Ctx) error {
    user, err := h.service.GetUser(c.UserContext(), userID)
    if err != nil {
        return err
    }
    return response.Success(c, user)
}
```

Apply these rules consistently:

- Boundary methods in services, ports, and repositories accept `ctx context.Context`, normally as their first argument after the receiver.
- Repositories call GORM with `db.WithContext(ctx)` or use the equivalent context-aware SQL APIs.
- Services may derive child contexts for a narrower deadline or cancellation scope, but must preserve the parent context and call the derived cancel function.
- Request handlers must not use `context.Background()` or `context.TODO()` for request work.
- Process-level startup and shutdown code may create a root context with `signal.NotifyContext`; that context must not replace an individual request context.
- Context values are reserved for request-scoped metadata such as request IDs and authenticated claims, not for passing business parameters.

CPU-only adapters such as JWT verification may not need context internally, but if they are exposed as an application port, the port contract should still accept the request context so future implementations can perform context-aware work without changing service boundaries.

## Transaction management

Transaction boundaries belong to the use case, not to the HTTP handler and not to an individual repository method. A service decides which operations must succeed or fail atomically and asks a transaction-capable port to execute them.

The transaction abstraction must expose domain or application operations, never database handles. For example, a transaction manager may accept `ctx` and a callback receiving transaction-scoped ports or a repository facade:

```text
service(ctx)
  → transaction port: WithinTransaction(ctx, callback)
  → repository adapter starts GORM transaction
  → callback uses transaction-scoped ports
  → adapter commits on nil, rolls back on error or cancellation
```

Implementation rules:

- Define a focused transaction or unit-of-work port in `internal/core/ports` when a feature needs multiple atomic writes.
- Implement that port in `internal/repositories/postgres` using GORM's transaction support.
- Do not pass `*gorm.DB`, `*sql.Tx`, or repository implementation types into `internal/services`.
- The adapter owns begin, commit, rollback, and database-specific error translation.
- Context cancellation or a returned error must cause rollback; successful completion commits once.
- Keep one transaction boundary around the complete use case. Nested service calls should join the existing unit of work rather than silently opening independent transactions.
- A single read or write does not need a transaction solely because it uses a repository; use transactions for atomicity, consistency, or explicitly required isolation.

The current foundation has no multi-write feature use cases. Add the transaction port alongside the first feature that requires atomic state changes rather than introducing a generic transaction abstraction in advance.

## Validation boundary

Validation is intentionally split between transport concerns and business concerns.

### Format and syntax validation in `httpapi`

HTTP handlers validate whether a request can be decoded and understood as an HTTP message:

- JSON syntax and request body decoding.
- Required transport fields, primitive types, and basic length limits.
- Path and query parameter parsing.
- Header, content-type, URL, and email syntax where applicable.
- Authentication header shape and other protocol-level requirements.

Malformed input should be rejected before calling a service and returned through the standard error envelope with a stable client-safe error code.

### Business and semantic validation in `services`

Services validate whether a well-formed request is allowed and meaningful in the current domain state:

- Authorization and role-based business rules.
- Ownership and resource relationships.
- State-machine transitions such as accepting an offer or completing a job.
- Cross-field rules, uniqueness decisions, limits, and other domain invariants.
- Preconditions that require repository data.

Service validation must run even when the service is called by a non-HTTP adapter, test, or background worker. Repositories may enforce database constraints as a final integrity safeguard, but they must not be the only place where business rules are checked. Domain types should hold invariants that are intrinsic to the entity regardless of transport.

## Request flow

A normal feature request follows this path:

```text
Client
  → Fiber middleware
  → httpapi handler (c.UserContext())
  → service/use case (ctx)
  → port interface (ctx)
  → PostgreSQL repository (ctx)
  → GORM/database (context-aware)
```

The response travels back through the same layers. Database errors should be logged with context and translated into safe application errors before reaching the client. Internal SQL details and secrets must never be returned in an API response.

## Adding a new feature

Use this sequence for a new feature:

1. Define or update the domain entity and constrained values.
2. Add the database migration in `migrations` before writing repository queries.
3. Define the smallest required port in `internal/core/ports`.
4. Specify context propagation and, if needed, the transaction boundary in the port contract.
5. Implement the business use case in `internal/services`, including semantic validation.
6. Implement the PostgreSQL repository in `internal/repositories/postgres`.
7. Add transport DTOs, syntax validation, and handlers in `internal/handlers/http`.
8. Register routes and inject dependencies through `internal/server`.
9. Document the endpoint in `docs/openapi.yaml`.
10. Add tests at the domain, service, repository, and handler levels as appropriate, including cancellation and rollback cases where relevant.

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

Boundary tests should also verify that request cancellation reaches the repository and that a failed multi-write use case rolls back all changes. Validation tests should cover both malformed transport input and semantically invalid but well-formed commands.

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
- Starting request work with `context.Background()` or discarding the context passed by the caller.
- Leaking `*gorm.DB`, `*sql.Tx`, or GORM-specific transaction callbacks into services.
- Putting business validation only in a handler or repository, where non-HTTP callers can bypass it.
- Opening a separate repository transaction for each operation in a use case that must be atomic.

## Current status

The architecture foundation is complete, but feature repositories, services, handlers, authentication endpoints, and domain workflows are still future work. The current live endpoints are `/healthz` and `/readyz`.
