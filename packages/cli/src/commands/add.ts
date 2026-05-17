import { isContainerRunning } from "../lib/docker";
import { log, postgresEnv, redisEnv, rabbitmqEnv, aistorEnv, signozEnv } from "../lib/output";
import { loadRegistry, saveRegistry } from "../lib/registry";
import type { ServiceName } from "../lib/types";
import { validateAppName } from "../lib/validate";
import * as postgres from "../services/postgres";
import * as redis from "../services/redis";
import * as rabbitmq from "../services/rabbitmq";
import * as aistor from "../services/aistor";
import * as signoz from "../services/signoz";

const SERVICE_CONTAINERS: Record<ServiceName, string> = {
  postgres: "infra-postgres",
  redis: "infra-redis",
  rabbitmq: "infra-rabbitmq",
  aistor: "infra-aistor",
  signoz: "infra-signoz-otel-collector",
};

const SERVICE_START_HINT: Partial<Record<ServiceName, string>> = {
  signoz: "make signoz-up",
};

export async function addCommand(
  appName: string,
  services: ServiceName[],
): Promise<void> {
  if (!appName) {
    log.error("App name is required.");
    process.exit(1);
  }

  try {
    validateAppName(appName);
  } catch {
    log.error('Invalid app name. Use lowercase letters, numbers, and hyphens.');
    process.exit(1);
  }

  if (services.length === 0) {
    log.error("At least one service is required. Use -s aistor,...");
    process.exit(1);
  }

  const registry = await loadRegistry();
  const app = registry.apps[appName];
  if (!app) {
    log.error(`App "${appName}" not found. Create it first with: bun shelf setup ${appName} -s ...`);
    process.exit(1);
  }

  const toProvision: ServiceName[] = [];
  for (const service of services) {
    if (app.services[service]) {
      log.warn(`${service} already provisioned for "${appName}" — skipping`);
    } else {
      toProvision.push(service);
    }
  }

  if (toProvision.length === 0) {
    log.info("Nothing to do.");
    return;
  }

  for (const service of toProvision) {
    const container = SERVICE_CONTAINERS[service];
    if (!(await isContainerRunning(container))) {
      const hint = SERVICE_START_HINT[service] ?? "make up";
      log.error(`Container "${container}" is not running. Start it with: ${hint}`);
      process.exit(1);
    }
  }

  const results: string[] = [];

  for (const service of toProvision) {
    try {
      switch (service) {
        case "postgres": {
          const config = await postgres.provision(appName);
          app.services.postgres = config;
          results.push(postgresEnv(config));
          break;
        }
        case "redis": {
          const config = await redis.provision(appName);
          app.services.redis = config;
          results.push(redisEnv(config));
          break;
        }
        case "rabbitmq": {
          const config = await rabbitmq.provision(appName);
          app.services.rabbitmq = config;
          results.push(rabbitmqEnv(config));
          break;
        }
        case "aistor": {
          const config = await aistor.provision(appName);
          app.services.aistor = config;
          results.push(aistorEnv(config));
          break;
        }
        case "signoz": {
          const config = await signoz.provision(appName);
          app.services.signoz = config;
          results.push(signozEnv(config));
          break;
        }
      }
      log.success(`${service} provisioned`);
    } catch (err) {
      log.error(`Failed to provision ${service}: ${err instanceof Error ? err.message : err}`);
      process.exit(1);
    }
  }

  await saveRegistry(registry);

  console.log("");
  log.title(`Services attached to "${appName}":\n`);
  console.log(results.join("\n\n"));
  console.log("");
}
