.PHONY: up down test test-e2e test-all sqlc-gen pprof

up:
	docker compose up -d --build

down:
	docker compose down

test:
	go test ./... -count=1

e2e: up
	go test -tags=e2e -count=1 ./test/e2e/...
	make down

integration: 
	go test -tags=integration -count=1 ./...

test-all: test test-e2e integration

sqlc-gen:
	sqlc generate

pprof: 
	go tool pprof -http=:8081 http://localhost:8080/debug/pprof/profile?seconds=30