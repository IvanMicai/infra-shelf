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

| Serviço    | Acesso (rede Docker)  | Isolamento por app          |
|------------|-----------------------|-----------------------------|
| PostgreSQL | `postgres:5432`       | Database + user dedicado    |
| Redis      | `redis:6379`          | ACL user + prefixo de chave |
| RabbitMQ   | `rabbitmq:5672`       | Vhost + user dedicado       |

## CLI

```bash
bun shelf setup meu-app -s postgres,redis,rabbitmq   # Provisionar
bun shelf list                                         # Listar apps
bun shelf list --json                                  # Listar em JSON
bun shelf backup meu-app                               # Backup
bun shelf backup --all                                 # Backup de todos
bun shelf restore meu-app                              # Restaurar último backup
bun shelf restore meu-app --file backups/meu-app/postgres_20260405T0300.sql
bun shelf remove meu-app                               # Remover
bun shelf status                                       # Status dos containers
```

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
