APP_NAME := cuways-api

ifneq (,$(wildcard .env))
include .env
endif

DATABASE_URL ?= postgresql://myuser:mypassword@localhost:5432/cuway_database?sslmode=disable
MIGRATE := go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.3

.PHONY: run build test vet fmt db-up db-down migrate-up migrate-down migrate-version

run:
	go run ./cmd/api

build:
	go build -trimpath -o bin/$(APP_NAME) ./cmd/api

test:
	go test ./...

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
