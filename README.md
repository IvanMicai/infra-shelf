# infra-shelf

Base de infraestrutura compartilhada para desenvolvimento local. Um único Docker Compose com os serviços core, acessíveis exclusivamente via rede Docker `infra-shelf`.

A CLI gerencia o provisionamento de credenciais isoladas por app — cada app recebe seu próprio database, user e senha.

## Quick Start

```bash
cp .env.example .env
make build   # compila ./shelf e ./shelf-web (Go puro, sem CGO)
make up
```

## Serviços

| Serviço    | Acesso (rede Docker)              | Isolamento por app                       |
|------------|-----------------------------------|------------------------------------------|
| PostgreSQL | `postgres:5432`                   | Database + user dedicado                 |
| Redis      | `redis:6379`                      | ACL user + prefixo de chave              |
| RabbitMQ   | `rabbitmq:5672`                   | Vhost + user dedicado                    |
| AIStor     | `aistor:9000`                     | Bucket + access key dedicada             |
| SignOz     | `signoz-otel-collector:4317/4318` | `service.name` + resource attributes     |

## CLI

```bash
./shelf setup meu-app -s postgres,redis,rabbitmq,aistor,signoz   # Provisionar
./shelf list                                         # Listar apps
./shelf list --json                                  # Listar em JSON
./shelf backup meu-app                               # Backup
./shelf backup --all                                 # Backup de todos
./shelf restore meu-app                              # Restaurar último backup
./shelf restore meu-app --file backups/meu-app/postgres_20260405T0300.sql
./shelf remove meu-app                               # Remover
./shelf status                                       # Status dos containers
```

A CLI é um binário Go estático (sem CGO, sem runtime extra). `make cli` compila apenas ela; `make build` compila CLI + web juntos.

## Interface Web

A interface gráfica (`./shelf-web`) usa Go templates + HTMX. Ela compartilha o mesmo `internal/shelfcore` que a CLI — sem subprocess, sem parsing de stdout. SQLite (driver pure-Go `modernc.org/sqlite`) guarda schedules e histórico de execuções.

Na página de cada app, as credenciais ficam ocultas até acionar o botão `Reveal credentials`.

```bash
make app          # sobe o web em container
# ou rodar local: ./shelf-web
```

Por padrão ela sobe em `http://127.0.0.1:8080` com Basic Auth `admin` / `admin`. Configure `APP_USERNAME` e `APP_PASSWORD` antes de expor fora da máquina local.

Variáveis úteis:

```bash
APP_ADDR=127.0.0.1:8080
APP_USERNAME=admin
APP_PASSWORD=admin
APP_TIMEZONE=America/Sao_Paulo
APP_DATABASE_PATH=./data/app/infra-shelf-app.db
INFRA_SHELF_SECRET=gere-um-valor-longo-com-openssl-rand-base64-32
BACKUP_S3_BUCKET=meu-bucket
BACKUP_S3_REGION=us-east-1
BACKUP_S3_PREFIX=infra-shelf/backups
```

### Criptografia do registry

Por padrão, a CLI mantém compatibilidade com o registry antigo em texto claro. Para salvar credenciais criptografadas, defina `INFRA_SHELF_SECRET` no `.env` local e rode:

```bash
secret="$(openssl rand -base64 32)"
printf '\nINFRA_SHELF_SECRET=%s\n' "$secret" >> .env
./shelf registry encrypt
```

O formato AES-256-GCM é bit-compatível com o registry criptografado pela CLI TypeScript anterior — instalações existentes continuam funcionando sem ressetar o secret. Sem ele, a CLI e a interface web não conseguem revelar as credenciais já salvas. Novas alterações no registry continuam sendo salvas criptografadas enquanto o secret estiver definido.

### Backups no S3

A interface web pode enviar backups locais para S3 automaticamente depois de cada backup manual ou agendado. Configure no `.env`:

```bash
BACKUP_S3_BUCKET=meu-bucket
BACKUP_S3_REGION=us-east-1
BACKUP_S3_PREFIX=infra-shelf/backups
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
```

Para MinIO, LocalStack ou outro storage compatível com S3:

```bash
BACKUP_S3_ENDPOINT=http://localhost:9000
BACKUP_S3_FORCE_PATH_STYLE=true
```

Com `BACKUP_S3_BUCKET` configurado, backups novos vão para `s3://bucket/prefix/{app}/{arquivo}`. A tela de Backups também tem uma ação para enviar backups locais já existentes.

### Rotação de backups

Os schedules da interface web permitem configurar retenção por app:

- `Keep days`: remove backups mais antigos que esse número de dias.
- `Keep files`: mantém no máximo essa quantidade de arquivos por app/serviço.
- `0`: desativa aquela regra.

A rotação roda depois de backups agendados concluídos com sucesso. Quando S3 está configurado, a interface também tenta remover do bucket os objetos correspondentes aos arquivos locais apagados.

### Exemplo de output do setup

```
✔ postgres provisioned
✔ redis provisioned
✔ rabbitmq provisioned

App "meu-app" ready!

# === PostgreSQL ===
DATABASE_URL=postgres://meu-app:aBcDeFgH@postgres:5432/meu-app
DB_HOST=postgres
DB_PORT=5432
DB_USERNAME=meu-app
DB_PASSWORD=aBcDeFgH
DB_NAME=meu-app

# === Redis ===
REDIS_URL=redis://meu-app:xYzAbCdE@redis:6379/0
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_USERNAME=meu-app
REDIS_PASSWORD=xYzAbCdE
REDIS_PREFIX=meu-app:

# === RabbitMQ ===
AMQP_URL=amqp://meu-app:fGhIjKlM@rabbitmq:5672/meu-app
RABBITMQ_HOST=rabbitmq
RABBITMQ_PORT=5672
RABBITMQ_USERNAME=meu-app
RABBITMQ_PASSWORD=fGhIjKlM
RABBITMQ_VHOST=meu-app

# === AIStor (S3) ===
S3_ENDPOINT=http://aistor:9000
S3_BUCKET=meu-app
S3_REGION=us-east-1
S3_ACCESS_KEY_ID=meu-app
S3_SECRET_ACCESS_KEY=nOpQrStU
S3_FORCE_PATH_STYLE=true
AWS_ENDPOINT_URL=http://aistor:9000
AWS_ACCESS_KEY_ID=meu-app
AWS_SECRET_ACCESS_KEY=nOpQrStU
AWS_REGION=us-east-1

# === SignOz (OpenTelemetry) ===
OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_SERVICE_NAME=meu-app
OTEL_RESOURCE_ATTRIBUTES=service.name=meu-app,service.namespace=infra-shelf,deployment.environment=dev
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
```

## Conectar Outros Projetos

No `docker-compose.yml` do seu projeto, conecte à rede `infra-shelf` e use as connection strings geradas pela CLI:

```yaml
services:
  app:
    networks:
      - infra-shelf
    env_file:
      - .env  # cole as variáveis geradas pelo setup aqui

networks:
  infra-shelf:
    external: true
```

Os hostnames `postgres`, `redis` e `rabbitmq` são resolvidos automaticamente dentro da rede Docker.

## Observabilidade (SignOz)

Stack opt-in pra logs, traces e métricas centralizados. Ver detalhes em [`signoz/README.md`](signoz/README.md).

```bash
make signoz-up           # liga clickhouse + collector + UI (~1min no primeiro boot)
open http://localhost:3301
./shelf setup obs-test -s postgres,signoz   # provisiona com bloco OTEL pronto
```

Apps colam o bloco `# === SignOz (OpenTelemetry) ===` no `.env` e qualquer SDK OTel padrão (Python, Node, Go) detecta automaticamente o endpoint `http://signoz-otel-collector:4317`. Logs stdout dos containers `infra-*` já são coletados via collector (filtro por label `infra-shelf.observe=true`); métricas de container e do host também.

SignOz não tem backup per-app — telemetria fica no ClickHouse compartilhado e expira pela retention policy configurada na UI.

## Comandos Make

```bash
make build     # Compila ./shelf e ./shelf-web
make cli       # Apenas a CLI
make web       # Apenas o web
make test      # go test ./...
make up        # Iniciar todos os serviços
make down      # Parar todos os serviços
make restart   # Reiniciar todos os serviços
make status    # Ver status dos serviços
make logs      # Logs de todos os serviços
make logs-postgres  # Logs de um serviço específico
make network   # Ver containers na rede compartilhada
make reset     # Parar e apagar todos os dados
```

## Layout do código

```
cmd/
  shelf/         # CLI (Go binário estático)
  shelf-web/     # web server
internal/
  shelfcore/     # API compartilhada entre CLI e web (Setup/Add/Backup/...)
  registry/      # apps.json + cripto AES-256-GCM
  services/      # postgres / redis / rabbitmq / aistor / signoz
  cli/           # comandos cobra
  web/           # handlers, scheduler, backupservice, assets
  backup/  config/  docker/  envspec/  output/  passwordgen/  s3backup/
```
