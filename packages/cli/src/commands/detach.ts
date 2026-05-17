import { log } from "../lib/output";
import { loadRegistry, saveRegistry } from "../lib/registry";
import type { ServiceName } from "../lib/types";
import * as signoz from "../services/signoz";

// Addons can be detached without tearing anything down because they don't
// own per-app resources. Listed explicitly so we never accidentally detach
// a service that needs a real teardown (postgres, redis, ...).
const DETACHABLE: ReadonlySet<ServiceName> = new Set(["signoz"]);

export async function detachCommand(
  appName: string,
  services: ServiceName[],
): Promise<void> {
  if (!appName) {
    log.error("App name is required.");
    process.exit(1);
  }
  if (services.length === 0) {
    log.error("At least one service is required. Use -s signoz");
    process.exit(1);
  }

  for (const service of services) {
    if (!DETACHABLE.has(service)) {
      log.error(`"${service}" is not detachable. Use \`bun shelf remove <app>\` for full teardown.`);
      process.exit(1);
    }
  }

  const registry = await loadRegistry();
  const app = registry.apps[appName];
  if (!app) {
    log.error(`App "${appName}" not found.`);
    process.exit(1);
  }

  for (const service of services) {
    if (!app.services[service]) {
      log.warn(`${service} is not attached to "${appName}" — skipping`);
      continue;
    }

    if (service === "signoz") {
      await signoz.teardown(appName);
    }
    delete app.services[service];
    log.success(`${service} detached from ${appName}`);
  }

  await saveRegistry(registry);
}
