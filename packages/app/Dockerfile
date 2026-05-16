# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache build-base
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -o /out/infra-shelf-app ./cmd/infra-shelf-app

FROM oven/bun:1-alpine AS runtime
RUN apk add --no-cache docker-cli ca-certificates tzdata sqlite-libs
COPY --from=builder /out/infra-shelf-app /usr/local/bin/infra-shelf-app

WORKDIR /workspace
ENV INFRA_SHELF_ROOT=/workspace \
    APP_ADDR=0.0.0.0:8080 \
    APP_DATABASE_PATH=/workspace/data/app/infra-shelf-app.db \
    INFRA_SHELF_BACKUPS_DIR=/workspace/backups

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/infra-shelf-app"]
