# CU Ways Backend Guide

## Scope

This repository contains the CU Ways Go/Fiber backend. It owns HTTP behavior, application use cases, domain models, PostgreSQL persistence, migrations, authentication, and backend tests.

## Development guidance

- Read `README.md` and `docs/architecture.md` before changing backend structure.
- Keep business rules in the domain and service layers rather than in HTTP handlers or database adapters.
- Follow the existing separation between domain models, ports, services, repositories, handlers, and server composition.
- Use the existing response, error, logging, authentication, and configuration conventions.
- Treat migrations as the database schema history and review both the up and down direction where applicable.
- Keep handlers thin and validate external input at the application boundary.
- Add focused tests for new business behavior and regression cases. Reuse existing test helpers and patterns.
- Update the backend's implementation documentation or OpenAPI description when an externally visible behavior changes.

These are guidelines, not a reason to force a large refactor. When a nearby pattern is imperfect, prefer a small consistent improvement and mention larger follow-up work separately.

## Useful checks

Use the commands in the README/Makefile as the source of truth. Common checks include:

```powershell
go test ./...
go vet ./...
go build -trimpath ./cmd/api
```

Run formatting on changed Go files and verify migrations against a local database when a schema change is involved.

## Boundaries and safety

- Do not use database auto-migration as a substitute for versioned migrations.
- Do not expose passwords, tokens, or local `.env` values in code, logs, fixtures, or documentation.
- Do not store survey questions or responses in this service unless the project scope explicitly changes.
- Preserve unrelated working-tree changes and do not push or commit unless requested.
- When a permission, status transition, payment rule, or data-retention decision is unclear, document the assumption and ask for clarification before locking it into the model.
