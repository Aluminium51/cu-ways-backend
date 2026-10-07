# Backend verification

Use the native Go commands below from the backend repository. They are suitable for the workspace's diagnosis, investigation, verification, and implementation loop. Save focused evidence for the behavior being changed, then run the complete gates before handing over the work.

## Default gates

```powershell
go vet ./...
go test -json -count=1 ./...
go build -trimpath -o bin/cuways-api.exe ./cmd/api
```

Use `bin/cuways-api` as the output path on Linux/macOS. `-count=1` disables test result caching and `-json` emits per-package and per-test events, including skipped PostgreSQL cases. A passing default suite with skipped integration tests does not prove persistence behavior. Normal Go commands do not load `.env`; the application startup command does.

`make verify` provides vet, uncached tests, and the API build. CI additionally runs race detection, coverage, formatting, the existing linter, PostgreSQL integration tests, and builds all commands. It retains `coverage/tests.jsonl` and coverage evidence for seven days, including failed runs.

Formatting should be checked on current Go sources, including new files. Windows checkouts may contain CRLF even when the committed source is formatted. Compare LF-normalized source with `gofmt` output so line endings alone do not cause a formatting failure; run `gofmt -w` only on files being changed.

## HTTP composition smoke

```powershell
go test -json -count=1 -run '^TestWorkflowSmoke$' ./internal/server
```

`TestWorkflowSmoke` builds the actual Fiber application with a fake readiness port and no database connection. It verifies:

- `/healthz` succeeds even when the readiness dependency fails, without pinging it.
- `/readyz` returns the documented success or `503 database_unavailable` envelope.
- Anonymous GET requests to users, marketer profile, services, statistics, marketer search, and survey routes stop at authentication with `401 unauthorized` before persistence.
- Unknown routes use the standard `404 not_found` envelope.
- The `X-Request-ID` travels into structured diagnostics and dependency errors stay out of response bodies.

The evidence reports method, path, observed status, request ID, and dependency probe count. This proves HTTP composition and the anonymous authentication boundary. Authenticated feature behavior is covered by service/handler tests; PostgreSQL behavior requires integration tests. This smoke does not start the application process or exercise a browser.

## Optional live probe

After starting the API normally, probe the process explicitly:

```powershell
go run ./cmd/smoke -url http://127.0.0.1:8081
go run ./cmd/smoke -url http://127.0.0.1:8081 -readiness -timeout 5s
```

The command uses only `GET /healthz` and, when requested, `GET /readyz`. It accepts loopback HTTP(S) origins, refuses credentials/path/query/fragment values and redirects, and never loads `.env`, logs response bodies, authenticates, starts services, migrates, or seeds. Each JSON line reports the request ID, timing, observed status, and result without exposing a raw URL or underlying transport error.

The smoke binary exits with code `0` when the selected contracts passed, code `1` for invalid arguments or a contract failure, and code `2` when the local server could not be reached or the documented readiness response says the database is unavailable. Report code `2` as inconclusive. With `go run`, Go reports the binary's nonzero exit status in diagnostic output and may itself exit with code `1`; automation should use the compiled smoke binary or read the JSON result. The default workspace workflow uses the in-process smoke and does not require this live probe.

## PostgreSQL integration

Explicitly create and migrate a disposable local database named, for example, `cuways_test`. Keep this separate from the database used by the application. Set its URL for the test process:

```powershell
$env:TEST_DATABASE_URL = 'postgresql://test_user:test_password@127.0.0.1:5432/cuways_test?sslmode=disable'
go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.3 -path ./migrations -database $env:TEST_DATABASE_URL up
go test -json -count=1 ./internal/repositories/postgres
Remove-Item Env:TEST_DATABASE_URL
```

The example credentials describe a disposable fixture, not application credentials. `make test-integration` also requires `TEST_DATABASE_URL`. The URL guard allows only localhost/loopback, a database name ending in `_test`, and the optional `sslmode`/`connect_timeout` query parameters. It rejects ordinary application database names, remote hosts, and destination override parameters before any connection. A valid URL still requires a running PostgreSQL server and the correct migrations. Tests roll back fixture rows but can advance sequences, and migrations change the disposable database. Do not use a shared or production database.

CI provisions PostgreSQL 16 with a dedicated `cuways_test` database, applies versioned migrations using `DATABASE_URL`, and then uses `TEST_DATABASE_URL` for the tests. The application and production configuration continue to use `DATABASE_URL`.

## Architecture checks

```powershell
go test -count=1 ./internal/architecture
```

Production imports must point inward: services and ports cannot depend on HTTP or concrete PostgreSQL adapters, handlers cannot import database adapters, and platform code cannot depend on handlers/services. Domain imports are limited to the standard library and the existing exact-decimal value type. The parser includes production files for all platforms and excludes test files, which may import adapters to verify their contracts.

These checks enforce import structure. Context propagation, permissions, ownership, transactions, and other semantic behavior still need focused behavioral tests.
