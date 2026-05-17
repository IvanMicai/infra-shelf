import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { withTempRegistry, type RegistrySandbox } from "../helpers";

const E2E = process.env.INFRA_SHELF_E2E === "1";

// Each test isolates the registry under a temp path so the dev's real
// apps.json never gets touched. Containers (postgres/signoz) are shared
// global state — assume `make up` brought them up. Cleanup uses the CLI
// itself via dynamic import so we exercise the same code paths.
describe.skipIf(!E2E)("setup --envs full flow", () => {
  const APP_BASE = `e2etest-${Date.now()}`;
  let sandbox: RegistrySandbox;

  beforeEach(async () => {
    delete process.env.INFRA_SHELF_SECRET;
    sandbox = await withTempRegistry();
  });

  afterEach(async () => {
    // Best-effort cleanup — registry sandbox is destroyed, but real
    // containers still hold the provisioned DBs/users until we tear them
    // down. Fall back to direct docker commands if the CLI cleanup fails.
    try {
      const { removeCommand } = await import(`../../src/commands/remove?cache=${Math.random()}`);
      await removeCommand(`${APP_BASE}-staging`, true).catch(() => {});
      await removeCommand(`${APP_BASE}-production`, true).catch(() => {});
    } catch {}
    await sandbox.cleanup();
  });

  test("creates two siblings with shared service.name + per-env tag", async () => {
    const { setupCommand } = await import(`../../src/commands/setup?cache=${Math.random()}`);
    await setupCommand(APP_BASE, ["postgres", "signoz"], {
      envs: ["staging", "production"],
    });

    const { loadRegistry } = await import(`../../src/lib/registry?cache=${Math.random()}`);
    const registry = await loadRegistry();

    const staging = registry.apps[`${APP_BASE}-staging`];
    const production = registry.apps[`${APP_BASE}-production`];

    expect(staging).toBeDefined();
    expect(production).toBeDefined();

    expect(staging.services.postgres?.database).toBe(`${APP_BASE}-staging`);
    expect(production.services.postgres?.database).toBe(`${APP_BASE}-production`);

    expect(staging.services.signoz).toEqual({
      serviceName: APP_BASE,
      environment: "staging",
    });
    expect(production.services.signoz).toEqual({
      serviceName: APP_BASE,
      environment: "production",
    });

    // AppEntry persistence — survives detach/reattach
    expect(staging.environment).toBe("staging");
    expect(staging.signozServiceName).toBe(APP_BASE);
  });
});
