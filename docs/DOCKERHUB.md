# infra-shelf

One shared set of backing services for **all** your local projects, instead of a
`docker-compose.yml` per repository each running its own Postgres.

Start the stack once. Every project joins the same Docker network and reaches
services by hostname — `postgres`, `redis`, `rabbitmq`, `mongodb`. Each app gets
its **own database, user, vhost, bucket and generated password**, so no project
ever holds another's credentials, and no project holds a superuser.

- **Source:** <https://github.com/IvanMicai/infra-shelf>
- **Documentation:** <https://ivanmicai.github.io/infra-shelf/>
- **License:** MIT

## What is in this image

Two static Go binaries (pure Go, `CGO_ENABLED=0`, pure-Go SQLite) plus the
Docker CLI:

| Path | What it is |
| --- | --- |
| `/usr/local/bin/shelf-web` | Web UI on port 8080 — the image entrypoint |
| `/usr/local/bin/shelf` | CLI: provision apps, back up, restore, reconcile |

The image drives **sibling containers** through the mounted Docker socket; it is
not a database itself. It provisions and operates the services declared in the
project's compose files.

## Tags

Every release publishes `X.Y.Z`, `X.Y`, `X` and `latest`, built for
**`linux/amd64`** and **`linux/arm64`**.

Pin an exact version such as `0.2.0` for stable deployments and rollback. Use
`latest` only when the host should always pull the newest release.

## Running it

This image is designed to run as the `app` service of the project's compose
stack, because it needs two things mounted: the repository at `/workspace` (for
`.env`, the app registry and backups) and the Docker socket (to reach the
service containers).

```bash
git clone https://github.com/IvanMicai/infra-shelf
cd infra-shelf
make init                       # create .env from .env.example
docker compose pull app reconcile
docker compose up -d
```

The web UI is then on <http://127.0.0.1:8080> with Basic Auth `admin` / `admin`.
**Change `APP_USERNAME` and `APP_PASSWORD` in `.env` before exposing it beyond
your machine.**

Pin a release by setting `IMAGE_TAG` in `.env`:

```bash
IMAGE_TAG=0.2.0
```

### Provisioning an app

The CLI ships in the same image:

```bash
docker compose exec app shelf setup myapp -s postgres,redis,rabbitmq,mongodb
```

That creates the isolated resources and prints a ready-to-paste `.env` block of
connection strings. Reprint it any time with `shelf credentials myapp`.

### Standalone

```bash
docker run --rm \
  -v "$PWD:/workspace" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -p 8080:8080 \
  ivanmicai/infra-shelf:latest
```

## Services it manages

| Service | Network address | Per-app isolation |
| --- | --- | --- |
| PostgreSQL | `postgres:5432` | Dedicated database + user |
| Redis | `redis:6379` | ACL user + key prefix |
| RabbitMQ | `rabbitmq:5672` | Dedicated vhost + user |
| MongoDB | `mongodb:27017` | Dedicated database + user |
| S3 (MinIO/AIStor) | `aistor:9000` | Dedicated bucket + access key |
| SignOz | `signoz-otel-collector:4317/4318` | `service.name` + attributes |

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `APP_ADDR` | `0.0.0.0:8080` | Listen address |
| `APP_USERNAME` / `APP_PASSWORD` | `admin` / `admin` | Basic Auth for the UI |
| `APP_TIMEZONE` | `America/Sao_Paulo` | Timezone for schedules and timestamps |
| `INFRA_SHELF_ROOT` | `/workspace` | Mounted repository root |
| `INFRA_SHELF_REGISTRY_PATH` | `/workspace/data/apps.json` | App registry |
| `INFRA_SHELF_BACKUPS_DIR` | `/workspace/backups` | Local backup destination |
| `INFRA_SHELF_SECRET` | — | Encrypts the registry at rest (AES-256-GCM) |
| `BACKUP_S3_BUCKET` | — | Leave empty to keep backups local only |

Full reference: [docs/CONFIGURATION.md](https://github.com/IvanMicai/infra-shelf/blob/main/docs/CONFIGURATION.md).

## Features

- **Per-app isolation** — own database/user/vhost/bucket and a generated
  password; no shared superuser credentials handed to apps.
- **Backups and restore** — per app, per service, with retention, scheduling and
  optional upload to any S3-compatible storage.
- **Encrypted registry** — credentials stored AES-256-GCM encrypted at rest.
- **Reconcile** — rebuilds per-app resources from the registry after a volume
  loss; runs idempotently on every boot.
- **Opt-in S3 and observability** — object storage and a full SignOz stack only
  when you need them.

## Security

Defaults are development defaults. Before exposing anything beyond your machine,
change `APP_PASSWORD` and the service passwords in `.env`, and set
`INFRA_SHELF_SECRET`. See
[SECURITY.md](https://github.com/IvanMicai/infra-shelf/blob/main/SECURITY.md).
