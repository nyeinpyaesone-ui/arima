.PHONY: test vet fmt-check build run check

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
