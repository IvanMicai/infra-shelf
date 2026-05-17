# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build both binaries as static binaries (CGO disabled, modernc.org/sqlite is pure-Go).
RUN CGO_ENABLED=0 go build -trimpath -o /out/shelf ./cmd/shelf && \
    CGO_ENABLED=0 go build -trimpath -o /out/shelf-web ./cmd/shelf-web

FROM alpine:3.20 AS runtime
RUN apk add --no-cache docker-cli ca-certificates tzdata
COPY --from=builder /out/shelf /usr/local/bin/shelf
COPY --from=builder /out/shelf-web /usr/local/bin/shelf-web

WORKDIR /workspace
ENV INFRA_SHELF_ROOT=/workspace \
    APP_ADDR=0.0.0.0:8080 \
    APP_DATABASE_PATH=/workspace/data/app/infra-shelf-app.db \
    INFRA_SHELF_BACKUPS_DIR=/workspace/backups

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/shelf-web"]
