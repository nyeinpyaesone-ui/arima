FROM golang:1.26.11-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/erp-api ./cmd/erp-api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/outbox-worker ./cmd/outbox-worker

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/erp-api /erp-api
COPY --from=build /out/outbox-worker /outbox-worker
EXPOSE 8080 8081
USER nonroot:nonroot
ENTRYPOINT ["/erp-api"]
