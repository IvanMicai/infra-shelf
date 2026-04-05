.PHONY: up down restart status logs logs-% reset clean network help

ENV_FILE ?= .env

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

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

reset: ## Stop services and remove volumes (DESTROYS ALL DATA)
	@echo "WARNING: This will destroy all data in all volumes."
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	docker compose --env-file $(ENV_FILE) down -v

clean: reset ## Alias for reset

network: ## Show the shared network name and connected containers
	@. ./$(ENV_FILE) 2>/dev/null; \
		echo "Network: $${INFRA_NETWORK_NAME:-infra-shelf}"; \
		docker network inspect $${INFRA_NETWORK_NAME:-infra-shelf} \
			--format '{{range .Containers}}  - {{.Name}}{{"\n"}}{{end}}' 2>/dev/null \
			|| echo "  (network not created yet — run 'make up' first)"
