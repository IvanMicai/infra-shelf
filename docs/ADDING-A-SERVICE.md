# Adding a service

This is the recipe for teaching infra-shelf to provision a new backing service
(say, a hypothetical `valkey`). Services are wired into
`internal/shelfcore/services.go` through small maps and switch statements rather
than a Go interface, so "adding a service" means extending each dispatch point.

The cleanest worked example to copy is **`internal/services/mongodb/mongodb.go`**
— a complete, self-contained service with provision, teardown, backup, restore,
and an env block. Read it first.

## 1. The service package

Create `internal/services/<name>/<name>.go` exposing:

- `const Container = "infra-<name>"` — the container that hosts the admin tooling
  (must match the container name in `docker-compose.yml`).
- `func Provision(ctx, app string) (registry.<Name>Config, error)` — generate a
  password with `internal/passwordgen` and run the admin commands **inside the
  container** via `internal/docker` (`docker.Exec` / `docker.ExecWithStdin`).
  Create an isolated user + database/namespace scoped to `app`.
- `func Teardown(ctx, app string) error` — drop what `Provision` created.
- (If backupable) `Backup(...)` and `Restore(...)` — write/read a dump file. Use
  the server's own tools via `docker exec` (the existing services use `pg_dump`,
  `mongodump`, `rabbitmqctl export_definitions`, etc.).

Provisioners never use a Go client library — they shell into the container, so
whatever the server supports, infra-shelf supports. See
[Architecture → Services layer contract](ARCHITECTURE.md#services-layer-contract).

## 2. The registry type

In `internal/registry`:

- Add `type <Name>Config struct { … }` with the fields you stash (database,
  username, password, …). Keep JSON tags stable — the registry is persisted.
- Add a pointer field to the `Services` struct on `AppEntry`:
  `<Name> *<Name>Config` (nil = not provisioned).
- Add an env-block generator (mirror `postgresEnv`/`mongodbEnv`) and call it from
  `App.EnvFile()` so `shelf credentials` and `shelf setup` print the connection
  variables.

## 3. Wire the dispatch in `internal/shelfcore/services.go`

Extend each of these for the new service name:

| Symbol | What to add |
| --- | --- |
| `serviceContainer` | `"<name>": <name>.Container` |
| `startHint` | only if the service lives in an opt-in overlay (like `aistor`/`signoz`) |
| `nonBackupable` / `detachable` | only if applicable (e.g. telemetry-only addons) |
| `provisionService` | `case "<name>": return <name>.Provision(ctx, app)` |
| `teardownService` | `case "<name>": return <name>.Teardown(ctx, app)` |
| `setServiceConfig` / `clearServiceConfig` / `hasService` | a `case` mapping to `entry.Services.<Name>` |

If the service is backupable, also add its `case` to the dispatch in
`internal/shelfcore/backup.go` and `restore.go`.

## 4. Compose & environment

- Add the container to `docker-compose.yml` (image — **pin the tag**, volumes
  under `${DATA_DIR}`, the `infra-shelf` network, a healthcheck, and the
  `infra-shelf.observe=true` label if you want its logs in SignOz).
- Add any root credentials / ports to `.env.example` with safe local defaults.

## 5. Docs & tests

- Update the **README** services table and the valid-services list in
  [CLI.md](CLI.md) and the install/skill copy.
- Add a row to [CONFIGURATION.md](CONFIGURATION.md) for any new env vars.
- Add tests. At minimum, a table-driven test for the env-block output
  (mirror `internal/envspec/envspec_test.go`); provisioning/backup tests are
  expected for anything touching credentials (see CONTRIBUTING.md).

## 6. Verify

```bash
go vet ./... && go test ./... && make build
make up
./shelf setup demo -s <name>      # provisions cleanly, prints the env block
./shelf backup demo -s <name>     # if backupable
./shelf reconcile                 # idempotent — re-applies without error
./shelf remove demo -s <name>     # tears down cleanly
```
