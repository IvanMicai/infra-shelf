# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- One-command installer (`scripts/install.sh`) that clones, builds (host Go or a
  throwaway Docker build when Go is absent), and starts the core stack; plus a
  `make quickstart` target.
- Continuous integration under `.github/`: `vet`/`test`/build of both binaries,
  gitleaks secret scanning, Conventional-Commits PR-title linting, Dependabot, and
  issue/PR templates.
- Contributor and operator documentation in `docs/` (`ARCHITECTURE`,
  `CONFIGURATION`, `CLI`, `BACKUPS`, `OBSERVABILITY`, `ADDING-A-SERVICE`), and a
  trimmed, scannable README that links into it.
- Claude Code install skill (`.claude/skills/infra-shelf-install/`) and an
  `AGENTS.md` guide so an AI agent can install and wire infra-shelf into a project.

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
