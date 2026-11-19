.PHONY: test vet fmt-check build run check dev-config dev-up dev-worker dev-logs dev-migrate dev-down

test:
	go test ./...

vet:
	go vet ./...

fmt-check:
	@test -z "$$(gofmt -l ./cmd ./internal)" || (gofmt -l ./cmd ./internal; exit 1)

build:
	mkdir -p bin
	go build -trimpath -ldflags="-s -w" -o bin/erp-api ./cmd/erp-api

run:
	go run ./cmd/erp-api

check: fmt-check vet test build

# Local Docker Compose workflow. This creates only an ignored local .env file.
dev-config:
	@test -f .env || cp configs/arima.env.example .env
	docker compose config --quiet

dev-up: dev-config
	docker compose up --build -d api

dev-worker: dev-config
	docker compose -f compose.yaml -f compose.worker.yaml config --quiet
	docker compose -f compose.yaml -f compose.worker.yaml up --build -d api outbox-worker

dev-migrate: dev-config
	docker compose run --rm migrate

dev-logs:
	docker compose logs -f database migrate api

dev-down:
	docker compose down
