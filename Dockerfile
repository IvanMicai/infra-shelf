# syntax=docker/dockerfile:1.7

# The builder always runs on the *build* platform and cross-compiles for the
# target one: both binaries are pure Go (CGO disabled, modernc.org/sqlite), so a
# multi-arch build needs no QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -o /out/shelf ./cmd/shelf && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -o /out/shelf-web ./cmd/shelf-web

FROM alpine:3.20 AS runtime
RUN apk add --no-cache docker-cli ca-certificates tzdata
COPY --from=builder /out/shelf /usr/local/bin/shelf
COPY --from=builder /out/shelf-web /usr/local/bin/shelf-web

LABEL org.opencontainers.image.title="infra-shelf" \
      org.opencontainers.image.description="Shared local development infrastructure: PostgreSQL, Redis, RabbitMQ, MongoDB, S3 and SignOz with per-app isolation." \
      org.opencontainers.image.source="https://github.com/IvanMicai/infra-shelf" \
      org.opencontainers.image.licenses="MIT"

WORKDIR /workspace
ENV INFRA_SHELF_ROOT=/workspace \
    APP_ADDR=0.0.0.0:8080 \
    APP_DATABASE_PATH=/workspace/data/app/infra-shelf-app.db \
    INFRA_SHELF_BACKUPS_DIR=/workspace/backups

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/shelf-web"]
