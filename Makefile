.PHONY: up down test test-e2e test-all sqlc-gen

up:
	docker compose up -d --build

down:
	docker compose down

test:
	go test ./... -count=1

e2e: up
	go test -tags=e2e -count=1 ./test/e2e/...
	make down

test-all: test test-e2e

sqlc-gen:
	sqlc generate
