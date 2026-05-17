import { resolve } from "node:path";
import { mkdir } from "node:fs/promises";
import { log } from "../lib/output";
import { loadRegistry } from "../lib/registry";
import { isContainerRunning } from "../lib/docker";
import type { ServiceName } from "../lib/types";
import * as postgres from "../services/postgres";
import * as redis from "../services/redis";
import * as rabbitmq from "../services/rabbitmq";
import * as aistor from "../services/aistor";

const BACKUPS_DIR = resolve(process.cwd(), "backups");

const SERVICE_CONTAINERS: Record<ServiceName, string> = {
  postgres: "infra-postgres",
  redis: "infra-redis",
  rabbitmq: "infra-rabbitmq",
  aistor: "infra-aistor",
  signoz: "infra-signoz-otel-collector",
};

const SERVICE_EXT: Record<ServiceName, string> = {
  postgres: "sql",
  redis: "json",
  rabbitmq: "json",
  aistor: "tar",
  signoz: "",
};

// SignOz is a shared observability backend with no per-app data — backup
// is intentionally a no-op (telemetry rolls over in ClickHouse).
const NON_BACKUPABLE: ReadonlySet<ServiceName> = new Set(["signoz"]);

function timestamp(): string {
  return new Date().toISOString().replace(/[:.]/g, "").slice(0, 15);
}

async function backupApp(
  appName: string,
  services?: ServiceName[],
): Promise<void> {
  const registry = await loadRegistry();
  const app = registry.apps[appName];

  if (!app) {
    log.error(`App "${appName}" not found.`);
    process.exit(1);
  }

  const provisionedServices = (Object.keys(app.services) as ServiceName[]).filter(
    (s) => !NON_BACKUPABLE.has(s),
  );
  const targetServices = services?.length
    ? services.filter((s) => provisionedServices.includes(s) && !NON_BACKUPABLE.has(s))
    : provisionedServices;

  if (targetServices.length === 0) {
    log.error(`No matching services to backup for "${appName}".`);
    return;
  }

  const appDir = resolve(BACKUPS_DIR, appName);
  await mkdir(appDir, { recursive: true });

  const ts = timestamp();

  for (const service of targetServices) {
    const container = SERVICE_CONTAINERS[service];
    if (!(await isContainerRunning(container))) {
      log.error(`Container "${container}" is not running. Skipping ${service}.`);
      continue;
    }

    const fileName = `${service}_${ts}.${SERVICE_EXT[service]}`;
    const filePath = resolve(appDir, fileName);

    try {
      switch (service) {
        case "postgres":
          await postgres.backup(appName, filePath);
          break;
        case "redis":
          await redis.backup(appName, filePath);
          break;
        case "rabbitmq":
          await rabbitmq.backup(appName, filePath);
          break;
        case "aistor":
          await aistor.backup(appName, filePath);
          break;
      }
      log.success(`${service} -> ${fileName}`);
    } catch (err) {
      log.error(`Failed to backup ${service}: ${err instanceof Error ? err.message : err}`);
    }
  }
}

export async function backupCommand(
  appName: string | undefined,
  all: boolean,
  services?: ServiceName[],
): Promise<void> {
  if (all) {
    const registry = await loadRegistry();
    const appNames = Object.keys(registry.apps);

    if (appNames.length === 0) {
      log.info("No apps provisioned yet.");
      return;
    }

    for (const name of appNames) {
      log.title(`\nBacking up "${name}"...`);
      await backupApp(name, services);
    }
  } else {
    if (!appName) {
      log.error("App name is required. Use --all to backup all apps.");
      process.exit(1);
    }

    log.title(`Backing up "${appName}"...`);
    await backupApp(appName, services);
  }

  console.log("");
  log.success(`Backups saved to ${BACKUPS_DIR}/`);
}
