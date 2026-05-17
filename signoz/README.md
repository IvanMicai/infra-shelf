# SignOz (observabilidade)

Stack self-host de SignOz integrada ao infra-shelf. Opt-in: não sobe com `make up`.

## Subir

```bash
make signoz-up      # liga o stack (clickhouse, zookeeper, query-service, frontend, alertmanager, otel-collector)
make signoz-logs    # acompanha collector + query-service
make signoz-down    # para sem apagar dados
```

UI: <http://localhost:3301> (local) ou <http://192.168.15.4:3301> (TrueNAS).
Primeira tela pede pra criar admin (e-mail + senha) — fica salvo no SQLite do query-service.

> Primeiro boot demora ~1min: o ClickHouse cria schemas. Os healthchecks só ficam verdes depois disso.

## Conectar um app

```bash
bun shelf setup meu-app -s postgres,signoz
```

O bloco `# === SignOz (OpenTelemetry) ===` no output traz `OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317` e `OTEL_SERVICE_NAME=meu-app`. Cole no `.env` do app — qualquer SDK OTel padrão (Python, Node, Go, etc) detecta automaticamente.

App existente? `bun shelf add meu-app -s signoz`.

## O que é coletado automaticamente

- **Logs stdout** de todo container marcado com label `infra-shelf.observe=true` (postgres, redis, rabbitmq, aistor, app web)
- **Métricas de container** (CPU/RAM/rede/IO) via `docker_stats` receiver
- **Métricas de host** via `hostmetrics` receiver (reais no TrueNAS; container-view no macOS)
- **Traces, métricas e logs OTLP** que os apps mandarem

## Backup

Não há backup per-app. Telemetria fica no ClickHouse compartilhado e expira pela retention policy configurada na UI do SignOz (Settings → Retention). `bun shelf backup meu-app` simplesmente pula signoz.

## Versões

Tags pinadas no `docker-compose.signoz.yml`. Pra atualizar: bumpe as tags + leia release notes em <https://github.com/SigNoz/signoz/releases>. Stack atual reflete SignOz v0.55 community.

## Troubleshooting

- **UI vazia / "no data"**: confira `make signoz-logs` — o collector deve mostrar `Everything is ready. Begin running and processing data.`
- **App não aparece em Services**: confira que o endpoint do app é `http://signoz-otel-collector:4317` (na rede `infra-shelf`) ou `http://localhost:4317` (de fora). `OTEL_SERVICE_NAME` precisa estar setado.
- **Memória apertada**: ClickHouse + Zookeeper consomem ~2GB sozinhos. Em dev local considere `make signoz-down` quando não estiver usando.
