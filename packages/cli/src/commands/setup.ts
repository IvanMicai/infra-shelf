import { isContainerRunning } from "../lib/docker";
import { log, postgresEnv, redisEnv, rabbitmqEnv } from "../lib/output";
import { loadRegistry, saveRegistry } from "../lib/registry";
import type { ServiceName } from "../lib/types";
import { validateAppName } from "../lib/validate";
import * as postgres from "../services/postgres";
import * as redis from "../services/redis";
import * as rabbitmq from "../services/rabbitmq";

const SERVICE_CONTAINERS: Record<ServiceName, string> = {
  postgres: "infra-postgres",
  redis: "infra-redis",
  rabbitmq: "infra-rabbitmq",
};

export async function setupCommand(
  appName: string,
  services: ServiceName[],
  options?: { fullAccess?: boolean },
): Promise<void> {
  if (!appName) {
    log.error("App name is required.");
    process.exit(1);
  }

  try {
    validateAppName(appName);
  } catch {
    log.error('Invalid app name. Use lowercase letters, numbers, and hyphens (e.g., "my-app").');
    process.exit(1);
  }

  if (services.length === 0) {
    log.error("At least one service is required. Use -s postgres,redis,rabbitmq");
    process.exit(1);
  }

  const registry = await loadRegistry();

  if (registry.apps[appName]) {
    log.error(`App "${appName}" already exists. Remove it first with: bun shelf remove ${appName}`);
    process.exit(1);
  }

  // Check containers are running
  for (const service of services) {
    const container = SERVICE_CONTAINERS[service];
    if (!(await isContainerRunning(container))) {
      log.error(`Container "${container}" is not running. Start it with: make up`);
      process.exit(1);
    }
  }

  registry.apps[appName] = {
    createdAt: new Date().toISOString(),
    services: {},
  };

  const results: string[] = [];

  for (const service of services) {
    try {
      switch (service) {
        case "postgres": {
          const config = await postgres.provision(appName);
          registry.apps[appName].services.postgres = config;
          results.push(postgresEnv(config));
          break;
        }
        case "redis": {
          const config = await redis.provision(appName, { fullAccess: options?.fullAccess });
          registry.apps[appName].services.redis = config;
          results.push(redisEnv(config));
          break;
        }
        case "rabbitmq": {
          const config = await rabbitmq.provision(appName);
          registry.apps[appName].services.rabbitmq = config;
          results.push(rabbitmqEnv(config));
          break;
        }
      }
      log.success(`${service} provisioned`);
    } catch (err) {
      log.error(`Failed to provision ${service}: ${err instanceof Error ? err.message : err}`);
    }
  }

  await saveRegistry(registry);

  console.log("");
  log.title(`App "${appName}" ready!\n`);
  console.log(results.join("\n\n"));
  console.log("");
}
