.PHONY: help init quickstart up down restart status logs logs-% reset clean network app dev app-build app-logs s3-up s3-down s3-restart s3-logs s3-status signoz-up signoz-down signoz-restart signoz-logs signoz-status up-all build cli web test

ENV_FILE ?= .env
S3_COMPOSE := -f docker-compose.yml -f docker-compose.s3.yml
SIGNOZ_COMPOSE := -f docker-compose.yml -f docker-compose.signoz.yml
ALL_COMPOSE := -f docker-compose.yml -f docker-compose.s3.yml -f docker-compose.signoz.yml

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

init: ## Create .env from the example and prepare local data/backups dirs
	@test -f $(ENV_FILE) || cp .env.example $(ENV_FILE)
	@mkdir -p data backups
	@echo "Ready. Edit $(ENV_FILE), then run 'make build && make up'."

quickstart: init build up ## First run: create .env, build binaries, start the core stack
	@echo ""
	@echo "infra-shelf is up. Provision your first app with:"
	@echo "  ./shelf setup myapp -s postgres,redis,rabbitmq,mongodb"
	@echo "(remember to change the default passwords in .env first)"

build: cli web ## Build both binaries

cli: ## Build the shelf CLI binary (./shelf)
	CGO_ENABLED=0 go build -trimpath -o shelf ./cmd/shelf

web: ## Build the shelf-web binary (./shelf-web)
	CGO_ENABLED=0 go build -trimpath -o shelf-web ./cmd/shelf-web

test: ## Run the unit + integration test suite
	go test ./...

up: ## Start all services
	docker compose --env-file $(ENV_FILE) up -d

down: ## Stop all services
	docker compose --env-file $(ENV_FILE) down

restart: ## Restart all services
	docker compose --env-file $(ENV_FILE) restart

status: ## Show service status
	docker compose --env-file $(ENV_FILE) ps

logs: ## Tail logs for all services
	docker compose --env-file $(ENV_FILE) logs -f

logs-%: ## Tail logs for a specific service (e.g., make logs-postgres)
	docker compose --env-file $(ENV_FILE) logs -f $*

reset: ## Stop services and remove data (DESTROYS ALL DATA)
	@echo "WARNING: This will destroy all data."
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	docker compose --env-file $(ENV_FILE) down
	@. ./$(ENV_FILE) 2>/dev/null; rm -rf $${DATA_DIR:-./data}

clean: reset ## Alias for reset

network: ## Show the shared network name and connected containers
	@. ./$(ENV_FILE) 2>/dev/null; \
		echo "Network: $${INFRA_NETWORK_NAME:-infra-shelf}"; \
		docker network inspect $${INFRA_NETWORK_NAME:-infra-shelf} \
			--format '{{range .Containers}}  - {{.Name}}{{"\n"}}{{end}}' 2>/dev/null \
			|| echo "  (network not created yet — run 'make up' first)"

app: ## Start the web interface (docker compose, builds if needed)
	docker compose --env-file $(ENV_FILE) up -d --build app

dev: app ## Alias for `make app` — start the full stack including the web UI

app-build: ## Rebuild the web interface image
	docker compose --env-file $(ENV_FILE) build app

app-logs: ## Tail web interface logs
	docker compose --env-file $(ENV_FILE) logs -f app

s3-up: ## Start the opt-in S3 (MinIO/AIStor) service
	docker compose --env-file $(ENV_FILE) $(S3_COMPOSE) up -d aistor

s3-down: ## Stop the S3 service
	docker compose --env-file $(ENV_FILE) $(S3_COMPOSE) stop aistor

s3-restart: ## Restart the S3 service
	docker compose --env-file $(ENV_FILE) $(S3_COMPOSE) restart aistor

s3-logs: ## Tail S3 service logs
	docker compose --env-file $(ENV_FILE) $(S3_COMPOSE) logs -f aistor

s3-status: ## Show S3 service status
	docker compose --env-file $(ENV_FILE) $(S3_COMPOSE) ps

signoz-up: ## Start SignOz observability stack (clickhouse + collector + UI)
	docker compose --env-file $(ENV_FILE) $(SIGNOZ_COMPOSE) up -d \
		signoz-zookeeper signoz-clickhouse \
		signoz-schema-migrator-sync signoz-schema-migrator-async \
		signoz signoz-otel-collector

signoz-down: ## Stop SignOz observability stack
	docker compose --env-file $(ENV_FILE) $(SIGNOZ_COMPOSE) stop \
		signoz-otel-collector signoz signoz-clickhouse signoz-zookeeper

signoz-restart: ## Restart SignOz observability stack
	docker compose --env-file $(ENV_FILE) $(SIGNOZ_COMPOSE) restart \
		signoz-otel-collector signoz signoz-clickhouse signoz-zookeeper

signoz-logs: ## Tail collector + server logs
	docker compose --env-file $(ENV_FILE) $(SIGNOZ_COMPOSE) logs -f \
		signoz-otel-collector signoz

signoz-status: ## Show SignOz container status
	docker compose --env-file $(ENV_FILE) $(SIGNOZ_COMPOSE) ps

up-all: ## Start core stack + S3 + SignOz
	docker compose --env-file $(ENV_FILE) $(ALL_COMPOSE) up -d
