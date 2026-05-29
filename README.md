<div align="center">

  <h1>infra-shelf</h1>

  <p>One shared Docker Compose stack for all your local-dev projects — PostgreSQL, Redis, RabbitMQ, MongoDB, optional S3 storage and SignOz observability — with a Go CLI and web UI that provision <strong>isolated per-app credentials</strong>.</p>

  <p>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT" /></a>
    <img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.25+" />
    <img src="https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white" alt="Docker Compose" />
  </p>

</div>

---

## Table of Contents

- [Introduction](#introduction)
- [Features](#features)
- [Services](#services)
- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
- [Setup Output](#setup-output)
- [Web Interface](#web-interface)
- [Connecting Other Projects](#connecting-other-projects)
- [Backups & S3](#backups--s3)
- [Reconcile](#reconcile)
- [Observability (SignOz)](#observability-signoz)
- [Security Notes](#security-notes)
- [Make Commands](#make-commands)
- [Development](#development)
- [Credits](#credits)
- [License](#license)

## Introduction

When you run several projects locally, each one tends to spin up its own
PostgreSQL, Redis, and friends — duplicated containers, duplicated ports, and no
isolation between them. **infra-shelf** replaces that with a single shared stack
reachable only on a private Docker network (`infra-shelf`), plus a small Go
toolchain that carves out **isolated credentials per app**: a dedicated database
and user on PostgreSQL, an ACL user and key prefix on Redis, a vhost and user on
RabbitMQ, a database and user on MongoDB, a bucket and access key on S3, and a
`service.name` on SignOz.

Both the CLI (`./shelf`) and the web UI (`./shelf-web`) are static Go binaries
(no CGO, no runtime dependencies) built on the same `internal/shelfcore` package —
the web UI calls the same code the CLI does, with no subprocess or stdout parsing.

## Features

- **Per-app isolation** — each app gets its own database/user/vhost/bucket and a
  generated password; no shared superuser credentials handed to apps.
- **One shared network** — every project connects to the external `infra-shelf`
  Docker network and reaches services by hostname (`postgres`, `redis`, …).
- **Static Go CLI + web UI** — single binaries, pure-Go SQLite
  (`modernc.org/sqlite`), no CGO.
- **Backups & restore** — per-app, per-service backups with restore, optional
  upload to any S3-compatible storage, and scheduled backups with retention.
- **Encrypted registry** — credentials can be stored AES-256-GCM encrypted at
  rest via `INFRA_SHELF_SECRET`.
- **Reconcile** — rebuild per-app resources from the registry after a volume
  loss (server reset, accidental delete); runs idempotently on every boot.
- **Opt-in S3 storage** — bring up object storage only when you need it
  (`make s3-up`); defaults to open-source MinIO.
- **Opt-in observability** — a full SignOz stack (`make signoz-up`) collects
  logs, traces and metrics; apps get a ready-to-paste OpenTelemetry block.

## Services

| Service | Network address | Per-app isolation | Default |
| --- | --- | --- | --- |
| PostgreSQL | `postgres:5432` | Dedicated database + user | core (`make up`) |
| Redis | `redis:6379` | ACL user + key prefix | core (`make up`) |
| RabbitMQ | `rabbitmq:5672` | Dedicated vhost + user | core (`make up`) |
| MongoDB | `mongodb:27017` | Dedicated database + user | core (`make up`) |
| S3 (MinIO/AIStor) | `aistor:9000` | Dedicated bucket + access key | opt-in (`make s3-up`) |
| SignOz | `signoz-otel-collector:4317/4318` | `service.name` + resource attributes | opt-in (`make signoz-up`) |

## Requirements

- **Docker** and **Docker Compose v2** (Docker Engine 24+ / Compose 2.20+).
- **Go 1.25+** — only needed to build the `shelf` / `shelf-web` binaries locally
  (the web UI and `reconcile` containers build inside Docker).
- **Memory** — the core stack is light; the optional SignOz stack
  (ClickHouse + Zookeeper) adds roughly **2 GB**.
- **Platforms** — works on macOS (Docker Desktop / OrbStack) and Linux servers.
  MongoDB is pinned to `mongo:7.0.34` on purpose: 8.x adds a FATAL guard that
  refuses Linux kernel ≥ 6.19 (an io_uring bug, SERVER-121912), which breaks the
  local Docker VM. See the comment in `docker-compose.yml` for details.

## Quick Start

```bash
make init        # create .env from .env.example + local data/ and backups/ dirs
# edit .env — at minimum change the default passwords (see Security Notes)
make build       # compile ./shelf and ./shelf-web (pure Go, no CGO)
make up          # start the core stack (postgres, redis, rabbitmq, mongodb)
```

Provision an app:

```bash
./shelf setup myapp -s postgres,redis,rabbitmq,mongodb
```

Run `make help` to see every available target.

## CLI Reference

`./shelf` is a single static binary. `make cli` builds just the CLI; `make build`
builds the CLI and the web UI together.

| Command | Description |
| --- | --- |
| `shelf setup <app> -s <services>` | Provision resources for a new app. Flags: `-s/--services`, `--env`, `--envs` (CSV of sibling envs), `--full-access`. |
| `shelf add <app> -s <services>` | Attach more services to an existing app. |
| `shelf detach <app> -s <services>` | Detach an addon (e.g. `signoz`) from an app — registry-only, no teardown. |
| `shelf list [--json]` | List all provisioned apps; `--json` emits the raw registry. |
| `shelf credentials <app>` | Print the `.env` block (connection strings) for an app. |
| `shelf remove <app> [-s <services>] [--force]` | Remove an app's resources (or only specific services). |
| `shelf backup [<app>] [--all] [-s <services>]` | Back up one app, or `--all` apps. |
| `shelf backup delete <app> <file>` | Delete a specific backup file. |
| `shelf restore <app> [--file <path>] [-s <services>] [--force]` | Restore from the latest backup, or an explicit `--file`. |
| `shelf start [docker compose args...]` | Start the infrastructure (`docker compose up -d`). |
| `shelf status` | Show infrastructure container status. |
| `shelf schedule list` | List backup schedules (shared with the web UI). |
| `shelf schedule create <app> --cron "<expr>" [-s <services>] [-z <tz>] [--retention-days N] [--retention-count N] [--disabled]` | Create a backup schedule. |
| `shelf schedule pause\|resume\|delete <id>` | Manage an existing schedule. |
| `shelf registry encrypt` | Re-encrypt the registry in place using `INFRA_SHELF_SECRET`. |
| `shelf reconcile` | Re-apply per-app resources (vhosts, users, perms) from the registry. |

Common examples:

```bash
./shelf setup myapp -s postgres,redis,rabbitmq,mongodb   # provision
./shelf add myapp -s signoz                              # add observability later
./shelf list --json                                      # machine-readable registry
./shelf backup myapp                                     # back up one app
./shelf backup --all                                     # back up everything
./shelf restore myapp --file backups/myapp/postgres_20260405T0300.sql
./shelf status                                           # container health
```

## Setup Output

`shelf setup` prints a ready-to-paste `.env` block with the generated
credentials and connection strings:

```
✔ postgres provisioned
✔ redis provisioned
✔ rabbitmq provisioned

App "myapp" ready!

# === PostgreSQL ===
DATABASE_URL=postgres://myapp:aBcDeFgH@postgres:5432/myapp
DB_HOST=postgres
DB_PORT=5432
DB_USERNAME=myapp
DB_PASSWORD=aBcDeFgH
DB_NAME=myapp

# === Redis ===
REDIS_URL=redis://myapp:xYzAbCdE@redis:6379/0
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_USERNAME=myapp
REDIS_PASSWORD=xYzAbCdE
REDIS_PREFIX=myapp:

# === RabbitMQ ===
AMQP_URL=amqp://myapp:fGhIjKlM@rabbitmq:5672/myapp
RABBITMQ_HOST=rabbitmq
RABBITMQ_PORT=5672
RABBITMQ_USERNAME=myapp
RABBITMQ_PASSWORD=fGhIjKlM
RABBITMQ_VHOST=myapp

# === MongoDB ===
MONGODB_URL=mongodb://myapp:vWxYzAbC@mongodb:27017/myapp?authSource=myapp
MONGODB_HOST=mongodb
MONGODB_PORT=27017
MONGODB_USERNAME=myapp
MONGODB_PASSWORD=vWxYzAbC
MONGODB_DATABASE=myapp
```

S3 and SignOz produce their own blocks when you include `-s aistor` (requires
`make s3-up`) or `-s signoz` (requires `make signoz-up`):

```
# === S3 (MinIO/AIStor) ===
S3_ENDPOINT=http://aistor:9000
S3_BUCKET=myapp
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=myapp
S3_SECRET_ACCESS_KEY=nOpQrStU
S3_FORCE_PATH_STYLE=true

# === SignOz (OpenTelemetry) ===
OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_SERVICE_NAME=myapp
OTEL_RESOURCE_ATTRIBUTES=service.name=myapp,service.namespace=infra-shelf,deployment.environment=dev
```

You can reprint this block any time with `./shelf credentials myapp`.

## Web Interface

The web UI (`./shelf-web`) is built with Go templates and shares the same
`internal/shelfcore` package as the CLI — no subprocess, no stdout parsing. A
pure-Go SQLite database (`modernc.org/sqlite`) stores schedules and run history.
On each app's page, credentials stay hidden until you click **Reveal
credentials**.

```bash
make app          # run the web UI in a container (builds if needed)
# or run locally:
./shelf-web
```

By default it listens on `http://127.0.0.1:8080` with Basic Auth `admin` /
`admin`. **Change `APP_USERNAME` and `APP_PASSWORD` before exposing it beyond
your machine.** Useful variables:

```bash
APP_ADDR=127.0.0.1:8080
APP_USERNAME=admin
APP_PASSWORD=admin
APP_TIMEZONE=UTC
APP_DATABASE_PATH=./data/app/infra-shelf-app.db
INFRA_SHELF_SECRET=        # generate with: openssl rand -base64 32
```

## Connecting Other Projects

In your project's `docker-compose.yml`, join the external `infra-shelf` network
and feed in the variables `shelf setup` generated. A copy-pasteable snippet lives
in [`examples/docker-compose.yml`](examples/docker-compose.yml):

```yaml
services:
  app:
    image: your-app:latest
    env_file:
      - .env   # the block printed by `shelf setup` / `shelf credentials`
    networks:
      - infra-shelf

networks:
  infra-shelf:
    external: true
```

The hostnames `postgres`, `redis`, `rabbitmq`, `mongodb` (and `aistor` when S3 is
up) resolve automatically inside the network.

## Backups & S3

Backups are local by default, written under `backups/<app>/`. With an
S3-compatible target configured, the web UI can also upload them after every
manual or scheduled backup.

S3 is **opt-in** — bring up object storage only when you need it:

```bash
make s3-up        # start the S3 service (defaults to open-source MinIO)
./shelf add myapp -s aistor   # provision a bucket + access key for the app
```

By default the S3 overlay runs the open-source `minio/minio` image, which works
out of the box. To use a commercial AIStor build instead, set `S3_IMAGE` and a
license in `.env`:

```bash
S3_IMAGE=quay.io/minio/aistor/minio:latest
AISTOR_LICENSE=...            # or AISTOR_SUBNET_API_KEY=...
```

To upload backups to any S3-compatible storage, configure in `.env`:

```bash
BACKUP_S3_BUCKET=my-bucket
BACKUP_S3_REGION=us-east-1
BACKUP_S3_PREFIX=infra-shelf/backups
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
# For MinIO / LocalStack / other S3-compatible endpoints:
BACKUP_S3_ENDPOINT=http://localhost:9000
BACKUP_S3_FORCE_PATH_STYLE=true
```

**Retention** is configured per schedule in the web UI (or via
`shelf schedule create`): `retention-days` deletes backups older than N days,
`retention-count` keeps the last N files per app/service, and `0` disables a
rule. Rotation runs after a successful scheduled backup; when S3 is configured,
the matching remote objects are removed too.

## Reconcile

`shelf reconcile` re-applies the vhosts, users, and permissions recorded in
`data/apps.json` against the running containers. It rescues state when a volume
is lost (server reset, accidental delete) and runs automatically on every
`docker compose up` (the one-shot `reconcile` service) once PostgreSQL and
RabbitMQ are healthy. It is idempotent — a no-op when everything already matches.

## Observability (SignOz)

An opt-in stack for centralized logs, traces and metrics. See
[`signoz/README.md`](signoz/README.md) for details.

```bash
make signoz-up                              # clickhouse + collector + UI (~1 min first boot)
open http://localhost:3301
./shelf setup obs-test -s postgres,signoz   # provision with a ready OTEL block
```

Apps paste the `# === SignOz (OpenTelemetry) ===` block into their `.env` and any
standard OTel SDK (Python, Node, Go, …) auto-detects the
`http://signoz-otel-collector:4317` endpoint. The collector also scrapes stdout
logs of containers labeled `infra-shelf.observe=true`, plus container and host
metrics. SignOz has no per-app backup — telemetry lives in the shared ClickHouse
and expires via the retention policy configured in the UI.

## Security Notes

infra-shelf is built for local development on a trusted machine or private
network.

- **Change every default credential** in `.env` before exposing anything:
  `APP_USERNAME`/`APP_PASSWORD` (web UI), `POSTGRES_PASSWORD`, `REDIS_PASSWORD`,
  `RABBITMQ_DEFAULT_PASS`, `MONGO_INITDB_ROOT_PASSWORD`, `AISTOR_ROOT_PASSWORD`.
  Generate secrets with `openssl rand -base64 16`.
- **The web UI is a single Basic Auth gate** bound to `127.0.0.1` by default. Put
  it behind HTTPS, a VPN, or a trusted reverse proxy if it must be reachable off
  localhost.
- **The web UI and `reconcile` mount `/var/run/docker.sock`** — effectively root
  on the host. Run them only on a machine you trust.
- **Set `INFRA_SHELF_SECRET`** to store registry credentials AES-256-GCM
  encrypted at rest (`./shelf registry encrypt`).
- **Never commit** `.env`, `data/`, `backups/`, or `data/apps.json` — they hold
  real credentials. They are gitignored by default.

See [SECURITY.md](SECURITY.md) for the full posture and how to report issues.

## Make Commands

```bash
make init        # Create .env from the example + local data/ and backups/ dirs
make build       # Build ./shelf and ./shelf-web
make cli         # Build only the CLI
make web         # Build only the web UI
make test        # go test ./...
make up          # Start the core stack
make down        # Stop the core stack
make restart     # Restart the core stack
make status      # Show service status
make logs        # Tail logs for all services
make logs-postgres   # Tail logs for a specific service
make app         # Start the web UI (docker compose, builds if needed)
make s3-up       # Start the opt-in S3 service
make s3-down     # Stop the S3 service
make signoz-up   # Start the opt-in SignOz stack
make signoz-down # Stop the SignOz stack
make up-all      # Start the core stack + S3 + SignOz
make network     # Show the shared network and connected containers
make reset       # Stop services and DELETE all data
```

## Development

```bash
make build       # CLI + web UI
make cli         # CLI only
make web         # web UI only
make test        # go test ./...
go vet ./...
```

Code layout:

```
cmd/
  shelf/         # CLI (static Go binary)
  shelf-web/     # web server
internal/
  shelfcore/     # shared API between CLI and web (Setup/Add/Backup/...)
  registry/      # apps.json + AES-256-GCM crypto
  services/      # postgres / redis / rabbitmq / aistor / mongodb / signoz
  cli/           # cobra commands
  web/           # handlers, scheduler, backupservice, assets
  backup/  config/  docker/  envspec/  output/  passwordgen/  s3backup/
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## Credits

infra-shelf stands on the shoulders of these open-source projects:

| Project | Role |
| --- | --- |
| [PostgreSQL](https://www.postgresql.org/) | Relational database |
| [Redis](https://redis.io/) | Key-value store |
| [RabbitMQ](https://www.rabbitmq.com/) | Message broker |
| [MongoDB](https://www.mongodb.com/) | Document database |
| [MinIO](https://min.io/) | S3-compatible object storage |
| [SignOz](https://signoz.io/) | Observability (logs, traces, metrics) |
| [Cobra](https://github.com/spf13/cobra) | CLI framework |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | Pure-Go SQLite driver |
| [Go](https://go.dev/) | Language & runtime |

## License

MIT. See [LICENSE](LICENSE).
