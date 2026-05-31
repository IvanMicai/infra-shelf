# Agent guide

Pointers for AI agents (Claude Code and others) working with infra-shelf.

## Install / provision on a user's behalf

A ready-made Claude Code skill lives at
[`.claude/skills/infra-shelf-install/SKILL.md`](.claude/skills/infra-shelf-install/SKILL.md).
It walks through prerequisites, the one-command install, securing the default
passwords, provisioning an app with `shelf setup`, and joining the shared
`infra-shelf` Docker network. Other agents can read it as a plain Markdown
runbook.

Key facts to respect:

- The `shelf` binary needs the repo on disk (it locates the root via
  `docker-compose.yml` + `go.mod`), so installation **clones** the repo — a bare
  `go install` will not work.
- `make init` writes `.env` with **insecure default passwords** — change them
  before exposing anything.
- Use only documented commands; the source of truth is [`docs/CLI.md`](docs/CLI.md)
  or `./shelf <command> --help`. Valid services: `postgres`, `redis`, `rabbitmq`,
  `mongodb`, `aistor`, `signoz`.
- Never commit `.env`, `data/`, `backups/`, or `data/apps.json`.

## Working in this repo

- Build: `make build` · Test: `go test ./...` · Vet: `go vet ./...`
- Architecture overview: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).
- Contribution rules (Conventional Commits, English, scoped PRs):
  [`CONTRIBUTING.md`](CONTRIBUTING.md).
