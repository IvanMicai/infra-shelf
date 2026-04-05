import { loadRegistry } from "../lib/registry";
import { log, postgresEnv, redisEnv, rabbitmqEnv } from "../lib/output";

export async function listCommand(json: boolean): Promise<void> {
  const registry = await loadRegistry();
  const appNames = Object.keys(registry.apps);

  if (appNames.length === 0) {
    log.info("No apps provisioned yet.");
    return;
  }

  if (json) {
    console.log(JSON.stringify(registry.apps, null, 2));
    return;
  }

  for (const name of appNames) {
    const app = registry.apps[name];
    const services = Object.keys(app.services);
    const created = new Date(app.createdAt).toLocaleDateString();

    log.title(`${name}`);
    log.dim(`Created: ${created} | Services: ${services.join(", ")}\n`);

    const blocks: string[] = [];

    if (app.services.postgres) {
      blocks.push(postgresEnv(app.services.postgres));
    }
    if (app.services.redis) {
      blocks.push(redisEnv(app.services.redis));
    }
    if (app.services.rabbitmq) {
      blocks.push(rabbitmqEnv(app.services.rabbitmq));
    }

    console.log(blocks.join("\n\n"));
    console.log("");
  }
}
