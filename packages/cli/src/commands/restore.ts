import { resolve } from "node:path";
import { readdir } from "node:fs/promises";
import { log } from "../lib/output";
import { loadRegistry } from "../lib/registry";
import { isContainerRunning } from "../lib/docker";
import type { ServiceName } from "../lib/types";
import * as postgres from "../services/postgres";
import * as redis from "../services/redis";
import * as rabbitmq from "../services/rabbitmq";

const BACKUPS_DIR = resolve(process.cwd(), "backups");

const SERVICE_CONTAINERS: Record<ServiceName, string> = {
  postgres: "infra-postgres",
  redis: "infra-redis",
  rabbitmq: "infra-rabbitmq",
};

function detectService(fileName: string): ServiceName | null {
  if (fileName.startsWith("postgres_")) return "postgres";
  if (fileName.startsWith("redis_")) return "redis";
  if (fileName.startsWith("rabbitmq_")) return "rabbitmq";
  return null;
}

async function findLatestBackup(
  appDir: string,
  service: ServiceName,
): Promise<string | null> {
  try {
    const files = await readdir(appDir);
    const matching = files
      .filter((f) => f.startsWith(`${service}_`))
      .sort()
      .reverse();
    return matching[0] ? resolve(appDir, matching[0]) : null;
  } catch {
    return null;
  }
}

async function restoreService(
  service: ServiceName,
  appName: string,
  filePath: string,
): Promise<void> {
  switch (service) {
    case "postgres":
      await postgres.restore(appName, filePath);
      break;
    case "redis":
      await redis.restore(appName, filePath);
      break;
    case "rabbitmq":
      await rabbitmq.restore(appName, filePath);
      break;
  }
}

export async function restoreCommand(
  appName: string,
  services: ServiceName[] | undefined,
  filePath: string | undefined,
  force: boolean,
): Promise<void> {
  if (!appName) {
    log.error("App name is required.");
    process.exit(1);
  }

  const registry = await loadRegistry();
  const app = registry.apps[appName];

  if (!app) {
    log.error(`App "${appName}" not found.`);
    process.exit(1);
  }

  // Restore from a specific file
  if (filePath) {
    const resolvedPath = resolve(filePath);
    const file = Bun.file(resolvedPath);
    if (!(await file.exists())) {
      log.error(`File not found: ${resolvedPath}`);
      process.exit(1);
    }

    const fileName = resolvedPath.split("/").pop() ?? "";
    const service = detectService(fileName);
    if (!service) {
      log.error(`Cannot detect service from filename "${fileName}". Expected format: postgres_*, redis_*, rabbitmq_*`);
      process.exit(1);
    }

    if (!(await isContainerRunning(SERVICE_CONTAINERS[service]))) {
      log.error(`Container "${SERVICE_CONTAINERS[service]}" is not running.`);
      process.exit(1);
    }

    if (!force) {
      const readline = await import("node:readline");
      const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
      const answer = await new Promise<string>((r) =>
        rl.question(`Restore ${service} for "${appName}" from ${fileName}? [y/N] `, r),
      );
      rl.close();
      if (answer.toLowerCase() !== "y") {
        log.info("Cancelled.");
        return;
      }
    }

    await restoreService(service, appName, resolvedPath);
    log.success(`${service} restored from ${fileName}`);
    return;
  }

  // Restore latest backups for each service
  const provisionedServices = Object.keys(app.services) as ServiceName[];
  const targetServices = services?.length
    ? services.filter((s) => provisionedServices.includes(s))
    : provisionedServices;

  const appDir = resolve(BACKUPS_DIR, appName);
  const restorePlan: { service: ServiceName; file: string }[] = [];

  for (const service of targetServices) {
    const latest = await findLatestBackup(appDir, service);
    if (latest) {
      restorePlan.push({ service, file: latest });
    } else {
      log.warn(`No backup found for ${service}`);
    }
  }

  if (restorePlan.length === 0) {
    log.error("No backups found to restore.");
    return;
  }

  if (!force) {
    log.title(`\nRestore plan for "${appName}":`);
    for (const { service, file } of restorePlan) {
      console.log(`  ${service} <- ${file.split("/").pop()}`);
    }
    console.log("");

    const readline = await import("node:readline");
    const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
    const answer = await new Promise<string>((r) =>
      rl.question("Proceed with restore? [y/N] ", r),
    );
    rl.close();
    if (answer.toLowerCase() !== "y") {
      log.info("Cancelled.");
      return;
    }
  }

  for (const { service, file } of restorePlan) {
    if (!(await isContainerRunning(SERVICE_CONTAINERS[service]))) {
      log.error(`Container "${SERVICE_CONTAINERS[service]}" is not running. Skipping.`);
      continue;
    }

    try {
      await restoreService(service, appName, file);
      log.success(`${service} restored`);
    } catch (err) {
      log.error(`Failed to restore ${service}: ${err instanceof Error ? err.message : err}`);
    }
  }
}
