# SignOz (observability)

A self-hosted SignOz stack integrated with infra-shelf. Opt-in: it does **not**
start with `make up`.

## Start

```bash
make signoz-up      # bring up the stack (clickhouse, zookeeper, query-service, frontend, alertmanager, otel-collector)
make signoz-logs    # follow the collector + query-service
make signoz-down    # stop without deleting data
```

UI: <http://localhost:3301> (or `http://<host>:3301` when running on a remote
server). The first screen asks you to create an admin (email + password), stored
in the query-service SQLite.

> First boot takes ~1 min: ClickHouse creates its schemas. Healthchecks only go
> green after that.

## Connect an app

```bash
./shelf setup myapp -s postgres,signoz
```

The `# === SignOz (OpenTelemetry) ===` block in the output carries
`OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317` and
`OTEL_SERVICE_NAME=myapp`. Paste it into the app's `.env` — any standard OTel SDK
(Python, Node, Go, etc.) auto-detects it.

Already have an app? `./shelf add myapp -s signoz`.

## What is collected automatically

- **stdout logs** of every container labeled `infra-shelf.observe=true`
  (postgres, redis, rabbitmq, aistor, the web app)
- **Container metrics** (CPU/RAM/network/IO) via the `docker_stats` receiver
- **Host metrics** via the `hostmetrics` receiver (real on a Linux host;
  container-scoped on macOS)
- **Traces, metrics and logs (OTLP)** that apps send

## Backup

There is no per-app backup. Telemetry lives in the shared ClickHouse and expires
via the retention policy configured in the SignOz UI (Settings → Retention).
`./shelf backup myapp` simply skips signoz.

## Versions

Image tags are pinned in `docker-compose.signoz.yml`. To upgrade: bump the tags
and read the release notes at <https://github.com/SigNoz/signoz/releases>. The
current stack tracks the SignOz v0.124 community build.

## Troubleshooting

- **Empty UI / "no data"**: check `make signoz-logs` — the collector should print
  `Everything is ready. Begin running and processing data.` Note that this line
  alone is **not** proof: the collector prints it for the degraded config too
  (see below). The conclusive check is that it is listening —
  `docker exec infra-signoz-otel-collector bash -c 'exec 3<>/dev/tcp/127.0.0.1/4318'`
  (silence = ok), or just `docker ps`, since the healthcheck runs exactly that.
- **Every producer gets `connection refused` on 4317, and the collector looks
  healthy**: it is running the all-`nop` fallback. The collector adopts it when a
  pipeline fails to build — typically because ClickHouse was not up yet when it
  started, which `depends_on` does not prevent after a Docker daemon restart.
  It then serves no OTLP listener at all and logs nothing further.
  **A `docker restart` does not fix this**: the fallback is persisted in the
  `--copy-path` file, which survives a restart. Recreate the container instead:

  ```bash
  docker compose -f docker-compose.yml -f docker-compose.signoz.yml \
    up -d --force-recreate --no-deps signoz-otel-collector
  ```

  The wait-for-ClickHouse guard in `docker-compose.signoz.yml` keeps it from
  happening again; the healthcheck makes it visible if it ever does.
- **App missing from Services**: confirm the app's endpoint is
  `http://signoz-otel-collector:4317` (on the `infra-shelf` network) or
  `http://localhost:4317` (from outside). `OTEL_SERVICE_NAME` must be set.
- **Tight on memory**: ClickHouse + Zookeeper use ~2 GB on their own. For local
  dev, `make signoz-down` when you are not using it.
