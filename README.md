# infra-shelf

Base de infraestrutura compartilhada para desenvolvimento local. Um único Docker Compose com todos os serviços necessários, acessíveis exclusivamente via rede Docker.

Os serviços de dados (PostgreSQL, Redis, RabbitMQ) não expõem portas ao host — a conexão é feita apenas pela rede Docker `infra-shelf`. A CLI gerencia o provisionamento de credenciais isoladas por app.

## Quick Start

```bash
cp .env.example .env
bun install
make up
```

## Serviços

| Serviço      | Acesso                          | UI                      |
|--------------|---------------------------------|-------------------------|
| PostgreSQL   | `postgres:5432` (rede Docker)   | —                       |
| pgAdmin      | —                               | http://localhost:5050   |
| Redis        | `redis:6379` (rede Docker)      | —                       |
| RedisInsight | —                               | http://localhost:5540   |
| RabbitMQ     | `rabbitmq:5672` (rede Docker)   | http://localhost:15672  |

## CLI — Provisionar Apps

A CLI cria databases, users e vhosts isolados para cada projeto.

```bash
# Provisionar um app com os serviços desejados
bun shelf setup meu-app -s postgres,redis,rabbitmq

# Listar apps provisionados
bun shelf list

# Remover um app
bun shelf remove meu-app

# Ver status dos containers
bun shelf status
```

### Exemplo de output

```
✔ postgres provisioned
✔ redis provisioned
✔ rabbitmq provisioned

App "meu-app" ready!

  PostgreSQL:
    DATABASE_URL=postgres://meu-app:aBcDeFgH@postgres:5432/meu-app

  Redis:
    REDIS_URL=redis://redis:6379/1

  RabbitMQ:
    AMQP_URL=amqp://meu-app:xYzAbCdE@rabbitmq:5672/meu-app
```

## Conectar Outros Projetos

No `docker-compose.yml` do seu projeto, conecte à rede `infra-shelf` e use as connection strings geradas pela CLI:

```yaml
services:
  app:
    networks:
      - infra-shelf
    environment:
      DATABASE_URL: postgres://meu-app:SENHA@postgres:5432/meu-app
      REDIS_URL: redis://redis:6379/1
      AMQP_URL: amqp://meu-app:SENHA@rabbitmq:5672/meu-app

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
make reset     # Parar e apagar todos os dados (volumes)
```
