.PHONY: up down test test-e2e

up:
	docker compose up -d --build

down:
	docker compose down

test:
	go test ./... -count=1

e2e: up
	go test -tags=e2e ./test/e2e/...
	make down

test-all: test test-e2e
