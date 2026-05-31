<!--
PR title must follow Conventional Commits (it becomes the squash-merge commit):
  fix: ...  feat: ...  docs: ...  refactor: ...  chore: ...  ci: ...
See CONTRIBUTING.md → Commit Messages.
-->

## What & why

<!-- What does this change, and what problem does it solve? -->

## How to test

<!-- Commands a reviewer can run to verify. -->

## Checklist

- [ ] `go vet ./... && go test ./... && make build` passes locally
- [ ] Tests added/updated for provisioning, backup/restore, reconcile, registry, or env-block changes
- [ ] Docs updated (README / `docs/` / `.env.example`) for any change to CLI, env vars, services, or compose
- [ ] No secrets, `.env`, `data/`, `backups/`, `data/apps.json`, or built binaries committed
