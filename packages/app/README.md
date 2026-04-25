# infra-shelf app

Web UI for infra-shelf, built with Go templates and HTMX.

## Run

```bash
cd packages/app
APP_USERNAME=admin APP_PASSWORD=admin go run ./cmd/infra-shelf-app
```

Or from the repository root:

```bash
make app
```

The default address is `127.0.0.1:8080`.

## Configuration

| Variable | Default |
| --- | --- |
| `APP_ADDR` | `127.0.0.1:8080` |
| `APP_USERNAME` | `admin` |
| `APP_PASSWORD` | `admin` |
| `APP_TIMEZONE` | `America/Sao_Paulo` |
| `APP_DATABASE_PATH` | `data/app/infra-shelf-app.db` |
| `INFRA_SHELF_SECRET` | empty |
| `BACKUP_S3_BUCKET` | empty |
| `BACKUP_S3_REGION` | `us-east-1` |
| `BACKUP_S3_PREFIX` | `infra-shelf/backups` |
| `BACKUP_S3_ENDPOINT` | empty |
| `BACKUP_S3_FORCE_PATH_STYLE` | `false` |
| `AWS_ACCESS_KEY_ID` | empty |
| `AWS_SECRET_ACCESS_KEY` | empty |
| `AWS_SESSION_TOKEN` | empty |
| `INFRA_SHELF_ROOT` | auto-detected |
| `INFRA_SHELF_REGISTRY_PATH` | `packages/cli/src/apps.json` |
| `INFRA_SHELF_BACKUPS_DIR` | `backups` |
| `BUN_BIN` | `bun` |

Schedules and backup run history are stored in SQLite. Provisioning, backup,
restore, and removal currently call the existing CLI so both interfaces keep the
same behavior.

## Registry encryption

Set `INFRA_SHELF_SECRET` in the repository `.env` to make the CLI save
`packages/cli/src/apps.json` encrypted. To migrate an existing plaintext
registry:

```bash
cd ../..
secret="$(openssl rand -base64 32)"
printf '\nINFRA_SHELF_SECRET=%s\n' "$secret" >> .env
bun shelf registry encrypt
```

Keep the generated secret safe; the app and CLI need the same value to reveal
credentials later.

## S3 backups

Set `BACKUP_S3_BUCKET` to upload every new manual or scheduled backup to S3:

```bash
BACKUP_S3_BUCKET=my-bucket
BACKUP_S3_REGION=us-east-1
BACKUP_S3_PREFIX=infra-shelf/backups
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
```

Objects are stored as:

```txt
s3://my-bucket/infra-shelf/backups/{app}/{backup-file}
```

For S3-compatible storage:

```bash
BACKUP_S3_ENDPOINT=http://localhost:9000
BACKUP_S3_FORCE_PATH_STYLE=true
```

## Backup rotation

Each schedule can define retention per app:

- `Keep days`: delete backups older than this number of days.
- `Keep files`: keep at most this many files per app/service.
- `0`: disable that rule.

Rotation runs after successful scheduled backups. If S3 is enabled, the app also
tries to delete the matching uploaded objects for local files it prunes.
