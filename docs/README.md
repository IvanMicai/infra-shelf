# infra-shelf documentation

Detailed docs for using, operating, and contributing to infra-shelf. New here?
Start with the [project README](../README.md) and its Quick Start.

## Using it

- **[CLI reference](CLI.md)** — every `shelf` command and flag.
- **[Configuration](CONFIGURATION.md)** — full environment-variable reference.
- **[Backups & restore](BACKUPS.md)** — local backups, retention, S3 upload.
- **[Observability](OBSERVABILITY.md)** — the opt-in SignOz stack.

## Contributing

- **[Architecture](ARCHITECTURE.md)** — how the pieces fit and *why*; read this first.
- **[Adding a service](ADDING-A-SERVICE.md)** — the step-by-step recipe.
- **[CONTRIBUTING](../CONTRIBUTING.md)** — dev setup, PR and commit conventions.

## Project meta

- **[Security policy](../SECURITY.md)** — posture and how to report issues.
- **[Code of Conduct](../CODE_OF_CONDUCT.md)**
- **[Changelog](../CHANGELOG.md)**
- **[Screenshots](screenshots/)** — web UI.

## AI agents

infra-shelf ships a Claude Code skill at
[`.claude/skills/infra-shelf-install/`](../.claude/skills/infra-shelf-install/SKILL.md)
that walks an LLM agent through installing the stack and wiring it into a project.
See [`AGENTS.md`](../AGENTS.md).
