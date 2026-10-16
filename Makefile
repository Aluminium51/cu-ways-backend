APP_NAME := cuways-api

ifneq (,$(wildcard .env))
include .env
endif

DATABASE_URL ?= postgresql://myuser:mypassword@localhost:5432/cuway_database?sslmode=disable
MIGRATE := go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.3

.PHONY: run build test test-json test-integration smoke smoke-live verify vet fmt db-up db-down migrate-up migrate-down migrate-version seed-admin seed-mock-users

run:
	go run ./cmd/api

build:
	go build -trimpath -o bin/$(APP_NAME) ./cmd/api

test:
	go test -count=1 ./...

test-json:
	go test -json -count=1 ./...

test-integration:
	$(if $(strip $(TEST_DATABASE_URL)),,$(error TEST_DATABASE_URL is required; use a migrated disposable loopback database ending in _test))
	go test -json -count=1 ./internal/repositories/postgres

smoke:
	go test -json -count=1 -run '^TestWorkflowSmoke$$' ./internal/server

smoke-live:
	go run ./cmd/smoke

verify: vet test build

vet:
	go vet ./...

fmt:
	go fmt ./...

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

migrate-up:
	$(MIGRATE) -path ./migrations -database "$(DATABASE_URL)" up

migrate-down:
	$(MIGRATE) -path ./migrations -database "$(DATABASE_URL)" down 1

migrate-version:
	$(MIGRATE) -path ./migrations -database "$(DATABASE_URL)" version

seed-admin:
	go run ./cmd/seed-admin

seed-mock-users:
	go run ./cmd/seed-mock-users
