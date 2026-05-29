# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
follows [Semantic Versioning](https://semver.org/).

## [0.1.0] - 2026-05-28

Initial open-source release under the MIT license.

### Changed

- Rewrote the entire CLI and web UI from a Bun/TypeScript monorepo into a single
  static Go module. The CLI (`shelf`) and web UI (`shelf-web`) now share one
  `internal/shelfcore` package — no subprocess calls, no stdout parsing. The
  AES-256-GCM registry remains bit-compatible with the previous TypeScript CLI.

### Added

- Per-app MongoDB provisioning, backup, and reconcile.
- Opt-in SignOz observability stack (`make signoz-up`) with auto-generated
  OpenTelemetry env blocks and container/host metric collection.
- Opt-in S3 / object-storage overlay (`make s3-up`), defaulting to open-source
  MinIO with an `S3_IMAGE` override for commercial AIStor builds.
- Scheduled backups with retention (days / count) and optional upload to any
  S3-compatible storage.
- Web UI (Go templates, pure-Go SQLite) for apps, credentials, backups,
  and schedules.
- `reconcile` to rebuild per-app resources from the registry after a volume loss
  (runs idempotently on every `docker compose up`).
- `make init` for a one-step first-run setup.
