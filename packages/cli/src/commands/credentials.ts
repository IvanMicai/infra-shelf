import { log, postgresEnv, redisEnv, rabbitmqEnv, aistorEnv, signozEnv } from "../lib/output";
import { loadRegistry } from "../lib/registry";

// Prints the .env block for an app — same content the web UI shows under
// "Reveal credentials" and the same the `setup`/`add` output emits, but
// pullable on demand for an app that already exists.
export async function credentialsCommand(appName: string): Promise<void> {
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

  const blocks: string[] = [];
  if (app.services.postgres) blocks.push(postgresEnv(app.services.postgres));
  if (app.services.redis) blocks.push(redisEnv(app.services.redis));
  if (app.services.rabbitmq) blocks.push(rabbitmqEnv(app.services.rabbitmq));
  if (app.services.aistor) blocks.push(aistorEnv(app.services.aistor));
  if (app.services.signoz) blocks.push(signozEnv(app.services.signoz));

  if (blocks.length === 0) {
    log.warn(`App "${appName}" has no services attached.`);
    return;
  }

  console.log(blocks.join("\n\n"));
}
