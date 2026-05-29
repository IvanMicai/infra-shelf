# Contributing

Thanks for considering a contribution. infra-shelf aims to stay practical,
self-hostable, and easy to operate for local development.

## Development Setup

```bash
make build       # build ./shelf and ./shelf-web
make test        # go test ./...
go vet ./...
```

Bring the stack up and exercise your change end to end:

```bash
make init
make up
./shelf setup demo -s postgres,redis,rabbitmq,mongodb
```

Full validation before opening a PR:

```bash
go vet ./... && go test ./... && make build
```

## Pull Request Guidelines

- Use **English** for documentation, comments, issue titles, and user-facing copy.
- Keep changes scoped to one behavior or feature when possible.
- Use [Conventional Commit](https://www.conventionalcommits.org/) titles so
  releases can be versioned automatically.
- Include tests when changing provisioning, backup/restore, reconcile, the
  registry, or env-block generation.
- Update documentation when changing deployment, environment variables, CLI
  commands, services, or the compose layout.
- Do **not** commit local `.env`, `data/`, `backups/`, `data/apps.json`,
  generated binaries (`shelf`, `shelf-web`), logs, or editor/agent state
  (`.claude/`, `.idea/`, `.vscode/`). These are gitignored.

## Reporting Bugs

Please include:

- What you tried to do.
- Expected vs. actual behavior.
- Relevant logs from `docker compose logs` (or `make logs`).
- Host OS, Docker version, and whether the S3 (`make s3-up`) or SignOz
  (`make signoz-up`) overlays were running.

## Commit Messages

Release versions are calculated from commits merged into `main`:

- `fix: ...` / `perf: ...` create a patch release.
- `feat: ...` creates a minor release.
- `feat!: ...` or a `BREAKING CHANGE:` footer creates a major release.
- `docs: ...`, `test: ...`, `refactor: ...`, `chore: ...`, and `ci: ...` do not
  create a release by default.

When using squash merge, the pull request title must follow the same convention.

## Security Hygiene

This repo handles credentials. Consider enabling the optional
[gitleaks](https://github.com/gitleaks/gitleaks) pre-commit hook (see
[`.pre-commit-config.yaml`](.pre-commit-config.yaml)) to catch accidentally
staged secrets. See [SECURITY.md](SECURITY.md) for more.
