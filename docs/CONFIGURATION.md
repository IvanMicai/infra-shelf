# Configuration reference

Every setting is an environment variable. They are read from the process
environment first, then from the `.env` file at the repo root (created by
`make init` from [`.env.example`](../.env.example)). The shell environment always
wins over `.env`.

Defaults below are the **binary defaults** (`internal/config/config.go`); the
shipped `.env.example` sets the credential variables to insecure local values you
are expected to change.

> **Never commit `.env`.** It holds real credentials and is gitignored.

## Web UI & runtime

| Variable | Default | Description |
| --- | --- | --- |
| `APP_ADDR` | `127.0.0.1:8080` | Address the web UI listens on. Set to `0.0.0.0:8080` only behind a trusted proxy/VPN. |
| `APP_USERNAME` | `admin` | Web UI Basic Auth username. **Change before exposing.** |
| `APP_PASSWORD` | `admin` | Web UI Basic Auth password. **Change before exposing.** |
| `APP_TIMEZONE` | `America/Sao_Paulo` | Timezone for schedule evaluation and timestamps. Any IANA name. |
| `APP_DATABASE_PATH` | `<root>/data/app/infra-shelf-app.db` | Pure-Go SQLite DB for schedules and run history. |
| `INFRA_SHELF_ROOT` | _(auto)_ | Repo root. Auto-detected by walking up for `docker-compose.yml` + `go.mod`; set explicitly when running the binary from elsewhere. |
| `INFRA_SHELF_REGISTRY_PATH` | `<root>/data/apps.json` | Path to the app registry. A legacy `packages/cli/src/apps.json` is used if present. |
| `INFRA_SHELF_BACKUPS_DIR` | `<root>/backups` | Where backup files are written. |
| `INFRA_SHELF_SECRET` | _(empty)_ | If set, the registry is encrypted AES-256-GCM at rest. Generate with `openssl rand -base64 32`. See [Architecture → Registry](ARCHITECTURE.md#the-registry). |

## Core service credentials

Consumed by `docker-compose.yml` when the core stack boots. These are the
**root/superuser** credentials; per-app users are generated separately by
`shelf setup`.

| Variable | Default (`.env.example`) | Service |
| --- | --- | --- |
| `POSTGRES_USER` | `postgres` | PostgreSQL superuser |
| `POSTGRES_PASSWORD` | `postgres` | PostgreSQL superuser password |
| `REDIS_PASSWORD` | `redis` | Redis default user password |
| `RABBITMQ_DEFAULT_USER` | `rabbit` | RabbitMQ admin user |
| `RABBITMQ_DEFAULT_PASS` | `rabbit` | RabbitMQ admin password |
| `MONGO_INITDB_ROOT_USERNAME` | `mongo` | MongoDB root user |
| `MONGO_INITDB_ROOT_PASSWORD` | `mongo` | MongoDB root password |

## Data & network

| Variable | Default | Description |
| --- | --- | --- |
| `DATA_DIR` | `./data` | Host directory bind-mounted for all service volumes. |
| `INFRA_NETWORK_NAME` | `infra-shelf` | External Docker network every project joins. |

## S3 / object storage overlay (opt-in: `make s3-up`)

| Variable | Default | Description |
| --- | --- | --- |
| `S3_IMAGE` | `minio/minio` | Image for the object-storage service. Set to `quay.io/minio/aistor/minio:latest` for the commercial AIStor build. |
| `AISTOR_ROOT_USER` | `aistor` | Object-storage root access key. |
| `AISTOR_ROOT_PASSWORD` | `aistor-root-change-me` | Object-storage root secret. **Change it.** |
| `AISTOR_API_PORT` | `9000` | Host port for the S3 API. |
| `AISTOR_LICENSE` | _(empty)_ | Commercial AIStor only — offline license content. Leave empty for MinIO. |
| `AISTOR_SUBNET_API_KEY` | _(empty)_ | Commercial AIStor only — MinIO SUBNET key for online registration. |

## SignOz observability overlay (opt-in: `make signoz-up`)

| Variable | Default | Description |
| --- | --- | --- |
| `SIGNOZ_UI_PORT` | `3301` | SignOz web UI port. |
| `SIGNOZ_OTLP_GRPC_PORT` | `4317` | OTLP/gRPC ingest port. |
| `SIGNOZ_OTLP_HTTP_PORT` | `4318` | OTLP/HTTP ingest port. |
| `SIGNOZ_CLICKHOUSE_USER` | `signoz` | ClickHouse user. |
| `SIGNOZ_CLICKHOUSE_PASSWORD` | `signoz` | ClickHouse password. |
| `SIGNOZ_JWT_SECRET` | _(empty)_ | Signs SignOz UI session tokens. Generate with `openssl rand -base64 32`. |
| `SIGNOZ_DEFAULT_ENV` | `dev` | `deployment.environment` resource attribute stamped on telemetry. |

See [OBSERVABILITY.md](OBSERVABILITY.md) for the full stack.

## Backup upload to S3 (optional)

Leave `BACKUP_S3_BUCKET` empty to keep backups local only.

| Variable | Default | Description |
| --- | --- | --- |
| `BACKUP_S3_BUCKET` | _(empty)_ | Bucket for uploaded backups. Empty = local only. |
| `BACKUP_S3_REGION` | `us-east-1` | Region (falls back to `AWS_REGION`). |
| `BACKUP_S3_PREFIX` | `infra-shelf/backups` | Key prefix for uploaded objects. |
| `BACKUP_S3_ENDPOINT` | _(empty)_ | Custom endpoint for MinIO / LocalStack / other S3-compatible storage. |
| `BACKUP_S3_FORCE_PATH_STYLE` | `false` | Set `true` for MinIO and most non-AWS endpoints. |
| `AWS_ACCESS_KEY_ID` | _(empty)_ | Credentials for the backup bucket. |
| `AWS_SECRET_ACCESS_KEY` | _(empty)_ | — |
| `AWS_SESSION_TOKEN` | _(empty)_ | Optional, for temporary credentials. |

See [BACKUPS.md](BACKUPS.md).
