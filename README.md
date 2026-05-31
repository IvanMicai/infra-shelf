<div align="center">

  <h1>infra-shelf</h1>

  <p>One shared Docker Compose stack for all your local-dev projects — PostgreSQL, Redis, RabbitMQ, MongoDB, optional S3 storage and SignOz observability — with a Go CLI and web UI that provision <strong>isolated per-app credentials</strong>.</p>

  <p>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT" /></a>
    <a href="https://github.com/IvanMicai/infra-shelf/actions/workflows/ci.yml"><img src="https://github.com/IvanMicai/infra-shelf/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
    <img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white" alt="Go 1.25+" />
    <img src="https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white" alt="Docker Compose" />
  </p>

</div>

---

When you run several projects locally, each spins up its own PostgreSQL, Redis,
and friends — duplicated containers and ports, no isolation. **infra-shelf**
replaces that with one shared stack on a private Docker network (`infra-shelf`),
plus a small Go toolchain that carves out **isolated credentials per app**: a
dedicated database/user on PostgreSQL, an ACL user + key prefix on Redis, a vhost
+ user on RabbitMQ, a database + user on MongoDB, a bucket + access key on S3, and
a `service.name` on SignOz.

The CLI (`shelf`) and web UI (`shelf-web`) are static, CGO-free Go binaries built
on one shared `internal/shelfcore` package — the web calls the same code the CLI
does, with no subprocess or stdout parsing.

## Quick Start

**One command** (clones into `./infra-shelf`, builds, and starts the core stack):

```bash
curl -fsSL https://raw.githubusercontent.com/IvanMicai/infra-shelf/main/scripts/install.sh | bash
```

No Go toolchain? The installer builds the binaries inside Docker automatically.

**Already cloned?**

```bash
make quickstart   # = make init + build + up
```

**Manual:**

```bash
make init     # create .env from .env.example + local data/ and backups/ dirs
make build    # compile ./shelf and ./shelf-web (pure Go, no CGO)
make up       # start the core stack (postgres, redis, rabbitmq, mongodb)
```

Then change the default passwords in `.env` (see [Security](#security)) and
provision an app:

```bash
./shelf setup myapp -s postgres,redis,rabbitmq,mongodb
```

`shelf setup` prints a ready-to-paste `.env` block of connection strings; reprint
it any time with `./shelf credentials myapp`.

## Services

| Service | Network address | Per-app isolation | Default |
| --- | --- | --- | --- |
| PostgreSQL | `postgres:5432` | Dedicated database + user | core (`make up`) |
| Redis | `redis:6379` | ACL user + key prefix | core (`make up`) |
| RabbitMQ | `rabbitmq:5672` | Dedicated vhost + user | core (`make up`) |
| MongoDB | `mongodb:27017` | Dedicated database + user | core (`make up`) |
| S3 (MinIO/AIStor) | `aistor:9000` | Dedicated bucket + access key | opt-in (`make s3-up`) |
| SignOz | `signoz-otel-collector:4317/4318` | `service.name` + attributes | opt-in (`make signoz-up`) |

## Features

- **Per-app isolation** — each app gets its own database/user/vhost/bucket and a
  generated password; no shared superuser credentials handed to apps.
- **One shared network** — every project joins the external `infra-shelf` network
  and reaches services by hostname (`postgres`, `redis`, …).
- **Static Go CLI + web UI** — single binaries, pure-Go SQLite, no CGO.
- **Backups & restore** — per-app, per-service, with retention, scheduling, and
  optional upload to any S3-compatible storage.
- **Encrypted registry** — credentials can be stored AES-256-GCM encrypted at
  rest via `INFRA_SHELF_SECRET`.
- **Reconcile** — rebuild per-app resources from the registry after a volume loss;
  runs idempotently on every boot.
- **Opt-in S3 & observability** — bring up object storage (`make s3-up`) or a full
  SignOz stack (`make signoz-up`) only when you need them.

## Requirements

- **Docker** + **Docker Compose v2** (Docker Engine 24+ / Compose 2.20+).
- **Go 1.25+** — only to build the binaries locally. The one-command installer can
  build them inside Docker instead, so Go on the host is optional.
- **Memory** — the core stack is light; the optional SignOz stack adds ~2 GB.
- **Platforms** — macOS (Docker Desktop / OrbStack) and Linux. MongoDB is pinned
  to `mongo:7.0.34` on purpose (see the comment in `docker-compose.yml`).

## Common commands

```bash
./shelf setup <app> -s postgres,redis,rabbitmq,mongodb   # provision an app
./shelf add <app> -s signoz                              # attach a service later
./shelf list                                             # list apps
./shelf credentials <app>                                # reprint the .env block
./shelf backup <app>        # or --all                   # back up
./shelf status                                           # container health
make app                                                 # run the web UI
make help                                                # every make target
```

Full command reference → **[docs/CLI.md](docs/CLI.md)**.

## Web interface

```bash
make app    # run the web UI in a container (builds if needed)
```

Listens on `http://127.0.0.1:8080` with Basic Auth `admin` / `admin` by default.
Credentials stay hidden until you click **Reveal credentials**. **Change
`APP_USERNAME` / `APP_PASSWORD` before exposing it beyond your machine.**

## Connecting other projects

In your project's `docker-compose.yml`, join the external `infra-shelf` network
and feed in the variables `shelf setup` generated (full snippet in
[`examples/docker-compose.yml`](examples/docker-compose.yml)):

```yaml
services:
  app:
    image: your-app:latest
    env_file: [.env]   # the block printed by `shelf setup`
    networks: [infra-shelf]

networks:
  infra-shelf:
    external: true
```

Hostnames `postgres`, `redis`, `rabbitmq`, `mongodb` (and `aistor` when S3 is up)
resolve automatically inside the network.

## Security

infra-shelf is built for local development on a trusted machine or private
network.

- **Change every default credential** in `.env` before exposing anything
  (`APP_*`, `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `RABBITMQ_DEFAULT_PASS`,
  `MONGO_INITDB_ROOT_PASSWORD`, `AISTOR_ROOT_PASSWORD`). Generate secrets with
  `openssl rand -base64 16`.
- **The web UI is a single Basic Auth gate** bound to `127.0.0.1` by default. Put
  it behind HTTPS / a VPN / a trusted proxy to reach it off localhost.
- **The web UI and `reconcile` mount `/var/run/docker.sock`** — effectively root
  on the host. Run them only on a machine you trust.
- **Set `INFRA_SHELF_SECRET`** to encrypt registry credentials at rest.
- **Never commit** `.env`, `data/`, `backups/`, or `data/apps.json`. They are
  gitignored by default.

See [SECURITY.md](SECURITY.md) for the full posture and how to report issues.

## Documentation

| Doc | Contents |
| --- | --- |
| [docs/CLI.md](docs/CLI.md) | Every `shelf` command and flag |
| [docs/CONFIGURATION.md](docs/CONFIGURATION.md) | Full environment-variable reference |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | How it works and why — start here to contribute |
| [docs/BACKUPS.md](docs/BACKUPS.md) | Backups, restore, retention, S3 upload |
| [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) | The opt-in SignOz stack |
| [docs/ADDING-A-SERVICE.md](docs/ADDING-A-SERVICE.md) | Recipe for a new backing service |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Dev setup, PR & commit conventions |

**Installing with an AI agent?** infra-shelf ships a Claude Code skill at
[`.claude/skills/infra-shelf-install/`](.claude/skills/infra-shelf-install/SKILL.md)
— see [AGENTS.md](AGENTS.md).

## Credits

infra-shelf stands on the shoulders of
[PostgreSQL](https://www.postgresql.org/), [Redis](https://redis.io/),
[RabbitMQ](https://www.rabbitmq.com/), [MongoDB](https://www.mongodb.com/),
[MinIO](https://min.io/), [SignOz](https://signoz.io/),
[Cobra](https://github.com/spf13/cobra),
[modernc.org/sqlite](https://gitlab.com/cznic/sqlite), and [Go](https://go.dev/).

## License

MIT. See [LICENSE](LICENSE).
