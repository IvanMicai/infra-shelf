# Backups, restore & retention

Backups are **local by default**, written under `backups/<app>/` as
`<service>_<timestamp>.<ext>` (timestamps are compact ISO-8601, e.g.
`postgres_20260517T193045.sql`, so they sort chronologically). With an
S3-compatible target configured, the web UI also uploads them after every backup.

## What gets backed up

Each service defines its own dump format (`internal/services/<name>`):

| Service | Method | Format |
| --- | --- | --- |
| PostgreSQL | `pg_dump --clean --if-exists` | `.sql` |
| Redis | per-app key snapshot (`<app>:*`) | `.json` |
| RabbitMQ | definitions export (vhost/users/policies) | `.json` |
| MongoDB | `mongodump` of the app database | archive |
| S3 (MinIO/AIStor) | bucket object sync | — |
| SignOz | **not backed up** — telemetry-only, expires by retention | — |

## Manual backup & restore

```bash
shelf backup myapp                 # one app, all its services
shelf backup myapp -s postgres     # one app, one service
shelf backup --all                 # every provisioned app

shelf restore myapp                # latest backup per service
shelf restore myapp --file backups/myapp/postgres_20260405T0300.sql
shelf backup delete myapp postgres_20260405T0300.sql
```

Restore infers the service from the filename prefix when given a single
`--file`. `--force` skips the confirmation prompt.

## Scheduled backups & retention

Create schedules from the web UI or the CLI (both share the app SQLite database):

```bash
shelf schedule create myapp --cron "0 3 * * *" -z UTC --retention-days 14
shelf schedule list
shelf schedule pause <id>
```

**Retention** runs after a successful scheduled backup:

- `--retention-days N` — delete backups older than N days.
- `--retention-count N` — keep only the last N files per app/service.
- `0` disables that rule.

When S3 upload is configured, the matching remote objects are pruned too.

## Upload to S3-compatible storage

Configure the `BACKUP_S3_*` and `AWS_*` variables in `.env` (full list in
[CONFIGURATION.md](CONFIGURATION.md#backup-upload-to-s3-optional)):

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

Leave `BACKUP_S3_BUCKET` empty to keep backups local only. This is independent of
the `make s3-up` object-storage overlay — you can upload backups to any external
S3 endpoint without running the local one.
