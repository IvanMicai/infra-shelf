# Architecture

This document explains how infra-shelf is put together and **why** the major
decisions were made. It is aimed at contributors. For day-to-day usage see the
[README](../README.md) and the [CLI reference](CLI.md).

## The big picture

infra-shelf is a single Go module (`github.com/IvanMicai/infra-shelf`) that ships
**two binaries over one shared library**:

```
                    ┌─────────────────────────┐
   cmd/shelf  ───▶  │                         │
   (Cobra CLI)      │   internal/shelfcore    │  ──▶  internal/services/*
                    │   (Engine + ops)        │       (postgres, redis, rabbitmq,
   cmd/shelf-web ─▶ │                         │        mongodb, aistor, signoz)
   (HTTP server)    └───────────┬─────────────┘            │  docker exec
                                │                           ▼
                    internal/registry            running Docker containers
                    (apps.json, AES-256-GCM)     on the `infra-shelf` network
```

The CLI and the web UI **never shell out to each other** and never parse each
other's stdout. They both import `internal/shelfcore` and call the same Go
functions. This is the single most important design decision in the codebase —
everything else follows from it.

## The shelfcore boundary

`internal/shelfcore/engine.go` defines the library boundary:

- **`Engine`** bundles the dependencies every operation needs: a
  `*registry.Store`, a `BackupsDir`, and a `Reporter`. Construct one per process
  (CLI) or per HTTP request (web).
- **`Reporter`** is an inversion-of-control hook for progress messages
  (`Success`/`Error`/`Info`/`Warn`/`Title`). The CLI wires it to
  `internal/output` for ANSI-formatted terminal output; the web wires it to
  `internal/web/runlog` (a thread-safe in-memory buffer shown in the UI). A nil
  reporter becomes `Discard` (no-ops), so the library can emit progress without
  knowing anything about the presentation layer.
- **Sentinel errors** (`ErrAppNotFound`, `ErrAppAlreadyExists`, `ErrNoServices`,
  `ErrContainerNotRunning`, `ErrNonBackupable`, `ErrBackupNotFound`, …) let the
  HTTP layer map failures to status codes (404/409/400) with `errors.Is` instead
  of string-matching.

Operations live in sibling files — `setup.go`, `add.go`, `remove.go`,
`detach.go`, `backup.go`, `restore.go`, `credentials.go`, `reconcile.go` — and
all hang off `Engine`.

## A request, end to end

`shelf setup myapp -s postgres,redis`:

1. **`cmd/shelf/main.go`** runs `cli.NewRootCmd().Execute()`.
2. **`internal/cli/root.go`** registers the Cobra commands;
   `internal/cli/commands/setup.go` parses flags (`-s/--services`, `--env`,
   `--envs`, `--full-access`), builds an `Engine` (`buildEngine()` in
   `commands/common.go`), and calls `engine.SetupApp(ctx, name, SetupOptions{…})`.
3. **`internal/shelfcore/setup.go`** validates the app and services, checks the
   backing containers are running (`containerCheck` in `services.go`), then for
   each service calls `provisionService(ctx, svc, app, opts)`.
4. **`internal/shelfcore/services.go`** dispatches to the service package —
   e.g. `postgres.Provision(ctx, app)` — which generates a password
   (`internal/passwordgen`) and runs the admin commands **inside the container**
   via `internal/docker` (`docker exec <container> psql …`, `redis-cli ACL …`,
   `rabbitmqctl add_vhost …`, `mongosh …`, `mc admin …`).
5. Each provisioner returns a typed config (`registry.PostgresConfig`, …) that
   `setServiceConfig` stores on the app's `registry.AppEntry`.
6. The registry is saved atomically, and the `.env` block is rendered from the
   entry (`registry.App.EnvFile()`) and printed.

The web UI takes the identical path: an HTTP handler in `internal/web/server`
builds an `Engine` (with a `runlog` reporter) and calls the same `SetupApp`.

## The registry

`internal/registry` is the source of truth for provisioned state.

- **Shape** — `Registry{ Version int, Apps map[string]AppEntry }`. Each
  `AppEntry` records `CreatedAt`, `Environment`, `SignozServiceName`, and a
  `Services` struct that is a tagged union of pointers
  (`*PostgresConfig`, `*RedisConfig`, `*RabbitMQConfig`, `*MongoDBConfig`,
  `*AIStorConfig`, `*SignozConfig`) — a nil pointer means "not provisioned".
- **Encryption** — `store_save.go` writes atomically (temp file + rename). When
  `INFRA_SHELF_SECRET` is set (env var or `.env`), the JSON payload is encrypted
  with **AES-256-GCM**, keyed by `SHA-256(secret)`, and wrapped in an envelope
  with an `encrypted` flag. `Load()` auto-detects the flag and decrypts with the
  same KDF, so encrypted and plaintext registries are interchangeable as long as
  the secret is present. `shelf registry encrypt` re-writes an existing registry
  in encrypted form.
- **Path resolution** — `INFRA_SHELF_REGISTRY_PATH` wins; otherwise the legacy
  `packages/cli/src/apps.json` is used if it still exists (so installs migrated
  from the old TypeScript CLI keep working), falling back to `data/apps.json`.
  The on-disk format is **bit-compatible with the previous TS CLI**, which is why
  the KDF and envelope are fixed.

## Root discovery

`internal/config/config.go` (`resolveRoot`) finds the repo so the binaries can
locate compose files, `.env`, `data/`, and `backups/`:

1. `INFRA_SHELF_ROOT` if set, else
2. walk up from the working directory until a directory contains **both
   `docker-compose.yml` and `go.mod`**.

**Implication:** the binaries are not standalone — they must run from inside a
checkout (or with `INFRA_SHELF_ROOT` pointing at one). This is why installation
clones the repo rather than `go install`-ing a bare binary, and why
[`scripts/install.sh`](../scripts/install.sh) exists.

## The web server

`cmd/shelf-web/main.go` loads config, opens the SQLite database, and starts the
HTTP server plus a background scheduler:

- **`internal/web/server`** — handlers for apps, credentials, backups, and
  schedules, behind HTTP Basic Auth (`internal/web/auth`). Templates and static
  assets are served from `internal/web/assets`. Credentials are masked until the
  user clicks **Reveal credentials**.
- **`internal/web/scheduler`** — a `robfig/cron` manager that runs scheduled
  backups. Schedules and run history live in a **pure-Go SQLite** database
  (`modernc.org/sqlite`, no CGO), shared with the `shelf schedule` CLI commands.
- **`internal/web/backupservice`** — wraps `shelfcore` backup/restore for the
  web, including retention pruning and optional S3 mirroring
  (`internal/s3backup`).
- **`internal/web/runlog`** — the `Reporter` implementation that captures
  operation progress for display.

## Reconcile

`shelf reconcile` (`internal/shelfcore/reconcile.go`) re-applies every vhost,
user, database, and permission recorded in the registry against the running
containers. It is **idempotent** — a no-op when state already matches — so it can
run on every `docker compose up` (the one-shot `reconcile` service) to rebuild
per-app resources after a volume loss (server reset, accidental delete).

## Services layer contract

Each service is an independent package under `internal/services/<name>/` and is
wired into `internal/shelfcore/services.go` through small maps and switch
statements rather than a Go interface:

- `serviceContainer` — the container that hosts the admin tooling.
- `startHint` — how to bring up a missing container (overlays differ:
  `make s3-up`, `make signoz-up`).
- `nonBackupable` / `detachable` — behavioral flags (SignOz is telemetry-only:
  no per-app backup, and it can be detached).
- `provisionService` / `teardownService` / `setServiceConfig` /
  `clearServiceConfig` / `hasService` — the dispatch points a new service must
  extend.

See [ADDING-A-SERVICE.md](ADDING-A-SERVICE.md) for the step-by-step recipe.

## Key decisions & their rationale

- **Static, CGO-free binaries** — built with `CGO_ENABLED=0 -trimpath`. SQLite is
  `modernc.org/sqlite` (pure Go) so there is no libc dependency; the binaries run
  unmodified on Alpine and scratch images.
- **One library, two front-ends** — eliminates the class of bugs that come from
  one tool parsing another tool's text output, and keeps the CLI and web UI
  behaviorally identical.
- **Container-native provisioning** — provisioners run real admin commands
  (`psql`, `redis-cli`, `rabbitmqctl`, `mongosh`, `mc`) inside the service
  containers via `docker exec`. There are no service client libraries to keep in
  sync; whatever the server supports, infra-shelf supports.
- **Registry encryption is opt-in but bit-compatible** — credentials can be
  encrypted at rest without breaking installs migrated from the original TS CLI.
- **`mongo:7.0.34` is pinned exactly** — MongoDB 8.x adds a FATAL guard that
  refuses Linux kernel ≥ 6.19 (an io_uring bug, SERVER-121912) which breaks the
  local Docker VM. See the comment in `docker-compose.yml`.
- **SignOz/ClickHouse versions are pinned** in `docker-compose.signoz.yml` for
  stability; see [`signoz/README.md`](../signoz/README.md).
