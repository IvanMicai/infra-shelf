---
name: infra-shelf-install
description: >-
  Install infra-shelf and provision isolated per-app credentials for a project's
  local development stack — PostgreSQL, Redis, RabbitMQ, MongoDB, optional S3
  (MinIO) and SignOz observability — all on one shared Docker network. Use when
  the user wants a local dev database/cache/message-broker/object-store, asks to
  "set up infra-shelf", or wants to connect a project to shared local infra.
---

# Install infra-shelf & connect a project

infra-shelf is a shared Docker Compose stack plus a Go CLI (`shelf`) that gives
each app **isolated credentials** (its own DB/user/vhost/bucket) on shared
services. This skill installs it and wires it into the user's project.

## Before you start

The `shelf` binary needs the repo checked out on disk (it finds the repo by
walking up for `docker-compose.yml` + `go.mod`). So **install means clone**, not
`go install`. Run commands from inside the checkout, or set `INFRA_SHELF_ROOT`.

Confirm prerequisites — stop and tell the user if any is missing:

```bash
git --version
docker --version
docker compose version      # must be Compose v2
docker info                 # daemon must be running
```

## Step 1 — Install

Prefer the one-command installer (it builds inside Docker if Go isn't present):

```bash
curl -fsSL https://raw.githubusercontent.com/IvanMicai/infra-shelf/main/scripts/install.sh | bash
```

It clones to `./infra-shelf` (override with `--dir <path>` or `INFRA_SHELF_DIR`),
creates `.env`, builds `./shelf` + `./shelf-web`, and starts the core stack.

Equivalent manual path:

```bash
git clone https://github.com/IvanMicai/infra-shelf.git
cd infra-shelf
make quickstart            # = make init + build + up
```

## Step 2 — Secure the defaults (do this before exposing anything)

`make init` copies `.env.example` to `.env` with **insecure default passwords**.
Edit `.env` and change at least: `APP_USERNAME`/`APP_PASSWORD` (web UI),
`POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `RABBITMQ_DEFAULT_PASS`,
`MONGO_INITDB_ROOT_PASSWORD`. Generate values with `openssl rand -base64 16`.
If you change a core password after the stack is up, recreate it: `make down && make up`.

Optional: set `INFRA_SHELF_SECRET` (`openssl rand -base64 32`) to encrypt the
credential registry at rest, then run `./shelf registry encrypt`.

## Step 3 — Provision the user's app

Pick only the services the project needs:

```bash
./shelf setup <app> -s postgres,redis,rabbitmq,mongodb
```

This prints a ready-to-paste `.env` block of connection strings (DATABASE_URL,
REDIS_URL, AMQP_URL, MONGODB_URL, …). Reprint any time with
`./shelf credentials <app>`. Add services later with
`./shelf add <app> -s <services>`.

- S3 needs the overlay first: `make s3-up`, then `-s aistor`.
- Observability needs: `make signoz-up`, then `-s signoz`.

## Step 4 — Wire it into the user's project

Copy the printed block into the project's `.env`, then join the shared network in
the project's `docker-compose.yml` (see `examples/docker-compose.yml`):

```yaml
services:
  app:
    image: your-app:latest
    env_file: [.env]
    networks: [infra-shelf]

networks:
  infra-shelf:
    external: true
```

Inside the network, services resolve by hostname: `postgres`, `redis`,
`rabbitmq`, `mongodb` (and `aistor` when S3 is up).

## Useful follow-ups

```bash
./shelf list                 # all provisioned apps
./shelf status               # container health
./shelf backup <app>         # back up (see docs/BACKUPS.md)
make app                     # web UI at http://127.0.0.1:8080
```

## Rules

- Use **only** the real commands above — do not invent flags. The authoritative
  list is `docs/CLI.md` (or `./shelf <cmd> --help`).
- Valid services: `postgres`, `redis`, `rabbitmq`, `mongodb`, `aistor`, `signoz`.
- Never commit the user's `.env`, `data/`, `backups/`, or `data/apps.json`.
- Treat the web UI / `reconcile` as root-on-host (they mount the Docker socket) —
  only run them on a trusted machine.
