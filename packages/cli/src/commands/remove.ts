import { log } from "../lib/output";
import { loadRegistry, saveRegistry } from "../lib/registry";
import * as postgres from "../services/postgres";
import * as redis from "../services/redis";
import * as rabbitmq from "../services/rabbitmq";
import * as aistor from "../services/aistor";

export async function removeCommand(
  appName: string,
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

  if (!force) {
    const readline = await import("node:readline");
    const rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout,
    });
    const answer = await new Promise<string>((resolve) => {
      rl.question(
        `Remove all resources for "${appName}"? [y/N] `,
        resolve,
      );
    });
    rl.close();

    if (answer.toLowerCase() !== "y") {
      log.info("Cancelled.");
      return;
    }
  }

  if (app.services.postgres) {
    try {
      await postgres.teardown(appName);
      log.success("PostgreSQL resources removed");
    } catch (err) {
      log.error(`Failed to remove PostgreSQL: ${err instanceof Error ? err.message : err}`);
    }
  }

  if (app.services.redis) {
    try {
      await redis.teardown(appName);
      log.success("Redis resources removed");
    } catch (err) {
      log.error(`Failed to remove Redis: ${err instanceof Error ? err.message : err}`);
    }
  }

  if (app.services.rabbitmq) {
    try {
      await rabbitmq.teardown(appName);
      log.success("RabbitMQ resources removed");
    } catch (err) {
      log.error(`Failed to remove RabbitMQ: ${err instanceof Error ? err.message : err}`);
    }
  }

  if (app.services.aistor) {
    try {
      await aistor.teardown(appName);
      log.success("AIStor resources removed");
    } catch (err) {
      log.error(`Failed to remove AIStor: ${err instanceof Error ? err.message : err}`);
    }
  }

  delete registry.apps[appName];
  await saveRegistry(registry);

  log.success(`App "${appName}" removed.`);
}
