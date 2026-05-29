# Security

## Supported Versions

Security fixes target the current `main` branch unless a release branch is
created in the future.

## Reporting a Vulnerability

Please do not open a public issue for a suspected vulnerability. Use GitHub
private vulnerability reporting if it is enabled for the repository. If it is
not enabled, contact the repository owner through their GitHub profile and avoid
including exploit details in public comments.

## Deployment Security

infra-shelf is intended for local development on a trusted machine or private
network.

- **Change every default credential** in `.env` before exposing anything:
  `APP_USERNAME`/`APP_PASSWORD`, `POSTGRES_PASSWORD`, `REDIS_PASSWORD`,
  `RABBITMQ_DEFAULT_PASS`, `MONGO_INITDB_ROOT_PASSWORD`, `AISTOR_ROOT_PASSWORD`,
  `SIGNOZ_CLICKHOUSE_PASSWORD`. Generate secrets with `openssl rand -base64 16`.
- **Bind the web UI to localhost.** It is a single Basic Auth gate. Put it behind
  HTTPS, a VPN, or a trusted reverse proxy when it must be reachable off
  localhost.
- **The web UI and `reconcile` mount `/var/run/docker.sock`** — effectively root
  on the host. Run them only on a machine you trust, and never expose their port
  publicly.
- **Encrypt the registry at rest.** Set `INFRA_SHELF_SECRET` (`openssl rand
  -base64 32`) and run `./shelf registry encrypt` to store credentials
  AES-256-GCM encrypted.
- **Keep secrets out of git.** Never commit `.env`, `data/`, `backups/`, or
  `data/apps.json` — they hold real provider keys, licenses, and generated
  credentials. They are gitignored by default. Consider the optional
  [gitleaks](https://github.com/gitleaks/gitleaks) pre-commit hook
  ([`.pre-commit-config.yaml`](.pre-commit-config.yaml)).

## Known Security Posture

The web UI uses a single shared Basic Auth credential, not a multi-user
authentication system, and provides no per-user authorization. Use an external
identity-aware proxy if you need stronger access control.
