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

export interface SetupOptions {
  fullAccess?: boolean;
  // `envs` (plural) expands into one app per env (`<app>-<env>`).
  envs?: string[];
  // `env` (singular) stamps deployment.environment on a single app
  // without expanding. Useful when the app name already encodes the env.
  env?: string;
}

export async function setupCommand(
  appName: string,
  services: ServiceName[],
  options?: SetupOptions,
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
    log.error("At least one service is required. Use -s postgres,redis,rabbitmq,aistor");
    process.exit(1);
  }

  // Targets describe the (possibly multiple) app rows we'll create.
  // - `envs` (plural) → expand into siblings `<app>-<env>`.
  // - `env` (singular) → tag-only on a single app (no name change).
  // - neither → single app, default env.
  const targets: Array<{ name: string; signozServiceName: string; signozEnv?: string }> =
    options?.envs && options.envs.length > 0
      ? options.envs.map((env) => ({
          name: `${appName}-${env}`,
          signozServiceName: appName,
          signozEnv: env,
        }))
      : [{ name: appName, signozServiceName: appName, signozEnv: options?.env }];

  const registry = await loadRegistry();

  // Fail-fast: check every target app is free before mutating anything.
  for (const target of targets) {
    if (registry.apps[target.name]) {
      log.error(`App "${target.name}" already exists. Remove it first with: bun shelf remove ${target.name}`);
      process.exit(1);
    }
    try {
      validateAppName(target.name);
    } catch {
      log.error(`Invalid expanded app name "${target.name}". Check --envs values.`);
      process.exit(1);
    }
  }

  // Check containers are running (once — shared across all targets).
  for (const service of services) {
    const container = SERVICE_CONTAINERS[service];
    if (!(await isContainerRunning(container))) {
      const hint = SERVICE_START_HINT[service] ?? "make up";
      log.error(`Container "${container}" is not running. Start it with: ${hint}`);
      process.exit(1);
    }
  }

  for (const target of targets) {
    registry.apps[target.name] = {
      createdAt: new Date().toISOString(),
      services: {},
    };

    const results: string[] = [];

    for (const service of services) {
      try {
        switch (service) {
          case "postgres": {
            const config = await postgres.provision(target.name);
            registry.apps[target.name].services.postgres = config;
            results.push(postgresEnv(config));
            break;
          }
          case "redis": {
            const config = await redis.provision(target.name, { fullAccess: options?.fullAccess });
            registry.apps[target.name].services.redis = config;
            results.push(redisEnv(config));
            break;
          }
          case "rabbitmq": {
            const config = await rabbitmq.provision(target.name);
            registry.apps[target.name].services.rabbitmq = config;
            results.push(rabbitmqEnv(config));
            break;
          }
          case "aistor": {
            const config = await aistor.provision(target.name);
            registry.apps[target.name].services.aistor = config;
            results.push(aistorEnv(config));
            break;
          }
          case "signoz": {
            const config = await signoz.provision(target.name, {
              serviceName: target.signozServiceName,
              environment: target.signozEnv,
            });
            registry.apps[target.name].services.signoz = config;
            results.push(signozEnv(config));
            break;
          }
        }
        log.success(`${target.name}: ${service} provisioned`);
      } catch (err) {
        log.error(`Failed to provision ${service} for ${target.name}: ${err instanceof Error ? err.message : err}`);
      }
    }

    await saveRegistry(registry);

    console.log("");
    log.title(`App "${target.name}" ready!\n`);
    console.log(results.join("\n\n"));
    console.log("");
  }
}
