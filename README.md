# infra-shelf

Base de infraestrutura compartilhada para desenvolvimento local. Um único Docker Compose com os serviços core, acessíveis exclusivamente via rede Docker `infra-shelf`.

A CLI gerencia o provisionamento de credenciais isoladas por app — cada app recebe seu próprio database, user e senha.

## Quick Start

```bash
cp .env.example .env
bun install
make up
```

## Serviços

| Serviço    | Acesso (rede Docker)  | Isolamento por app             |
|------------|-----------------------|--------------------------------|
| PostgreSQL | `postgres:5432`       | Database + user dedicado       |
| Redis      | `redis:6379`          | ACL user + prefixo de chave    |
| RabbitMQ   | `rabbitmq:5672`       | Vhost + user dedicado          |
| AIStor     | `aistor:9000`         | Bucket + access key dedicada   |

## CLI

```bash
bun shelf setup meu-app -s postgres,redis,rabbitmq,aistor   # Provisionar
bun shelf list                                         # Listar apps
bun shelf list --json                                  # Listar em JSON
bun shelf backup meu-app                               # Backup
bun shelf backup --all                                 # Backup de todos
bun shelf restore meu-app                              # Restaurar último backup
bun shelf restore meu-app --file backups/meu-app/postgres_20260405T0300.sql
bun shelf remove meu-app                               # Remover
bun shelf status                                       # Status dos containers
```

## Interface Web

A interface grafica fica em `packages/app` e usa Go templates + HTMX. Ela le o
registry atual da CLI, chama a CLI para provisionar/remover/backup/restore e usa
SQLite para salvar schedules e historico de execucoes.
Na pagina de cada app, as credenciais ficam ocultas ate acionar o botao
`Reveal credentials`.

```bash
make app
```

Por padrao ela sobe em `http://127.0.0.1:8080` com Basic Auth
`admin` / `admin`. Configure `APP_USERNAME` e `APP_PASSWORD` antes de expor fora
da maquina local.

Variaveis uteis:

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

Por padrao, a CLI mantem compatibilidade com o registry antigo em texto claro.
Para salvar credenciais criptografadas, defina `INFRA_SHELF_SECRET` no `.env`
local e rode:

```bash
secret="$(openssl rand -base64 32)"
printf '\nINFRA_SHELF_SECRET=%s\n' "$secret" >> .env
bun shelf registry encrypt
```

Guarde esse secret: sem ele, a CLI e a interface web nao conseguem revelar as
credenciais ja salvas no registry criptografado. Novas alteracoes no registry
continuam sendo salvas criptografadas enquanto esse secret estiver definido.

### Backups no S3

A interface web pode enviar backups locais para S3 automaticamente depois de
cada backup manual ou agendado. Configure no `.env`:

```bash
BACKUP_S3_BUCKET=meu-bucket
BACKUP_S3_REGION=us-east-1
BACKUP_S3_PREFIX=infra-shelf/backups
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
```

Se voce usa MinIO, LocalStack ou outro storage compativel com S3, tambem pode
usar:

```bash
BACKUP_S3_ENDPOINT=http://localhost:9000
BACKUP_S3_FORCE_PATH_STYLE=true
```

Com `BACKUP_S3_BUCKET` configurado, backups novos sao enviados para
`s3://bucket/prefix/{app}/{arquivo}`. A tela de Backups tambem tem uma acao para
enviar backups locais ja existentes.

### Rotacao de backups

Os schedules da interface web permitem configurar retencao por app:

- `Keep days`: remove backups mais antigos que esse numero de dias.
- `Keep files`: mantem no maximo essa quantidade de arquivos por app/servico.
- `0`: desativa aquela regra.

A rotacao roda depois de backups agendados concluidos com sucesso. Quando S3
esta configurado, a interface tambem tenta remover do bucket os objetos
correspondentes aos arquivos locais apagados.

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

## Comandos Make

```bash
make up        # Iniciar todos os serviços
make down      # Parar todos os serviços
make restart   # Reiniciar todos os serviços
make status    # Ver status dos serviços
make logs      # Logs de todos os serviços
make logs-postgres  # Logs de um serviço específico
make network   # Ver containers na rede compartilhada
make reset     # Parar e apagar todos os dados
```
