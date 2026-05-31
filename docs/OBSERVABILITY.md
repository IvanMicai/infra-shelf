# Observability (SignOz)

infra-shelf ships an **opt-in** SignOz stack for centralized logs, traces, and
metrics. It does not start with `make up`.

```bash
make signoz-up      # clickhouse + zookeeper + collector + UI (~1 min first boot)
open http://localhost:3301
make signoz-down    # stop without deleting data
```

For operational details — start/stop, first-boot admin creation, what is
collected automatically, version pinning, and troubleshooting — see
**[`signoz/README.md`](../signoz/README.md)**. This page covers how observability
fits into the infra-shelf model.

## How apps connect

SignOz is an **addon**: it provisions no per-app database or user, only telemetry
configuration recorded in the registry.

```bash
shelf setup obs-test -s postgres,signoz   # new app
shelf add myapp -s signoz                 # existing app
shelf detach myapp -s signoz              # remove (registry-only, no teardown)
```

The generated `.env` block carries a ready OpenTelemetry config:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_SERVICE_NAME=myapp
OTEL_RESOURCE_ATTRIBUTES=service.name=myapp,service.namespace=infra-shelf,deployment.environment=dev
```

Any standard OTel SDK (Python, Node, Go, …) auto-detects the endpoint. Inside the
`infra-shelf` network use `http://signoz-otel-collector:4317`; from outside,
`http://localhost:4317`.

## Multi-environment apps

`shelf setup api --envs staging,prod -s signoz` creates `api-staging` and
`api-prod` that **share one `service.name`** (`api`) but differ by the
`deployment.environment` attribute — so they appear as one service with two
environments in the SignOz UI. Use `--env <name>` to tag a single app.

## What is collected automatically

The collector scrapes the stdout logs of containers labeled
`infra-shelf.observe=true` (postgres, redis, rabbitmq, aistor, the web app), plus
container metrics (`docker_stats`) and host metrics (`hostmetrics`). See
[`signoz/README.md`](../signoz/README.md#what-is-collected-automatically) for the
full list.

## No per-app backup

Telemetry lives in the shared ClickHouse and expires via the retention policy
configured in the SignOz UI (Settings → Retention). `shelf backup` skips SignOz.

> **Memory note:** ClickHouse + Zookeeper use ~2 GB on their own. Run
> `make signoz-down` when you are not using it on a memory-constrained machine.
