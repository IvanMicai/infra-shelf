# @infra-shelf/cli

CLI para provisionar e gerenciar recursos de infraestrutura (PostgreSQL, Redis, RabbitMQ) para seus projetos.

```
bun shelf <command> [options]
```

## Comandos

### `setup`

Provisiona recursos para um app nos servicos selecionados. Cria banco, usuario e credenciais isolados por app.

```bash
bun shelf setup <app> -s <services>
```

| Opcao | Alias | Tipo | Descricao |
|-------|-------|------|-----------|
| `--services` | `-s` | string | Servicos separados por virgula (obrigatorio) |

Servicos validos: `postgres`, `redis`, `rabbitmq`

O nome do app deve comecar com letra minuscula e conter apenas letras minusculas, numeros e hifens (`^[a-z][a-z0-9-]*$`).

**O que e provisionado por servico:**

| Servico | Recursos criados |
|---------|-----------------|
| PostgreSQL | Database + user com todas as permissoes |
| Redis | ACL user com prefixo `{app}:*` |
| RabbitMQ | Vhost + user com permissoes completas |

**Exemplo:**

```
$ bun shelf setup my-app -s postgres,redis

✔ postgres provisioned
✔ redis provisioned

App "my-app" ready!

# === PostgreSQL ===
DATABASE_URL=postgres://my-app:abc123@postgres:5432/my-app
DB_HOST=postgres
DB_PORT=5432
DB_USERNAME=my-app
DB_PASSWORD=abc123
DB_NAME=my-app

# === Redis ===
REDIS_URL=redis://my-app:xyz789@redis:6379/0
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_USERNAME=my-app
REDIS_PASSWORD=xyz789
REDIS_PREFIX=my-app:*
```

---

### `list`

Lista todos os apps provisionados com suas credenciais.

```bash
bun shelf list [--json]
```

| Opcao | Alias | Tipo | Descricao |
|-------|-------|------|-----------|
| `--json` | `-j` | boolean | Saida em formato JSON |

**Exemplo:**

```
$ bun shelf list

my-app
Created: 4/5/2026 | Services: postgres, redis

# === PostgreSQL ===
DATABASE_URL=postgres://my-app:abc123@postgres:5432/my-app
...
```

---

### `remove`

Remove todos os recursos de um app (databases, usuarios, vhosts, chaves).

```bash
bun shelf remove <app> [--force]
```

| Opcao | Alias | Tipo | Descricao |
|-------|-------|------|-----------|
| `--force` | `-f` | boolean | Pula a confirmacao |

Sem `--force`, pede confirmacao antes de remover.

**O que e removido por servico:**

| Servico | Acao |
|---------|------|
| PostgreSQL | Termina conexoes, dropa database e user |
| Redis | Deleta chaves com prefixo `{app}:*`, remove ACL user |
| RabbitMQ | Deleta user e vhost |

**Exemplo:**

```
$ bun shelf remove my-app

Remove all resources for "my-app"? [y/N] y
✔ PostgreSQL resources removed
✔ Redis resources removed
✔ App "my-app" removed.
```

---

### `backup`

Faz backup dos dados de um app (ou de todos).

```bash
bun shelf backup <app> [-s <services>]
bun shelf backup --all [-s <services>]
```

| Opcao | Alias | Tipo | Descricao |
|-------|-------|------|-----------|
| `--services` | `-s` | string | Filtrar servicos a serem salvos |
| `--all` | `-a` | boolean | Backup de todos os apps |

Backups sao salvos em `backups/{app}/` com o formato `{servico}_{timestamp}.{ext}`:

| Servico | Formato | Metodo |
|---------|---------|--------|
| PostgreSQL | `.sql` | `pg_dump --clean --if-exists` |
| Redis | `.json` | Lua script que exporta todas as chaves com prefixo do app |
| RabbitMQ | `.json` | `rabbitmqctl export_definitions` (filtrado pelo vhost) |

**Exemplo:**

```
$ bun shelf backup my-app

Backing up "my-app"...
✔ postgres -> postgres_20260405T143000.sql
✔ redis -> redis_20260405T143000.json

Backups saved to backups/
```

---

### `restore`

Restaura dados de um app a partir de backups.

**Restaurar o backup mais recente de cada servico:**

```bash
bun shelf restore <app> [-s <services>] [--force]
```

**Restaurar a partir de um arquivo especifico:**

```bash
bun shelf restore <app> --file <path> [--force]
```

| Opcao | Alias | Tipo | Descricao |
|-------|-------|------|-----------|
| `--services` | `-s` | string | Filtrar servicos a serem restaurados |
| `--file` | | string | Caminho de um arquivo de backup especifico |
| `--force` | `-f` | boolean | Pula a confirmacao |

Sem `--force`, mostra o plano de restauracao e pede confirmacao.

O servico e detectado pelo nome do arquivo (`postgres_*`, `redis_*`, `rabbitmq_*`).

**Exemplo (backup mais recente):**

```
$ bun shelf restore my-app

Restore plan for "my-app":
  postgres <- postgres_20260405T143000.sql
  redis <- redis_20260405T143000.json

Proceed with restore? [y/N] y
✔ postgres restored
✔ redis restored
```

**Exemplo (arquivo especifico):**

```
$ bun shelf restore my-app --file backups/my-app/postgres_20260401T120000.sql

Restore postgres for "my-app" from postgres_20260401T120000.sql? [y/N] y
✔ postgres restored from postgres_20260401T120000.sql
```

---

### `status`

Mostra o estado dos containers de infraestrutura.

```bash
bun shelf status
```

**Exemplo:**

```
$ bun shelf status

Infrastructure Status

  PostgreSQL     🟢 running
  Redis          🟢 running
  RabbitMQ       🔴 exited
```

| Icone | Significado |
|-------|-------------|
| 🟢 | Container rodando |
| 🔴 | Container parado/com erro |
| ⏹ | Container nao criado |
