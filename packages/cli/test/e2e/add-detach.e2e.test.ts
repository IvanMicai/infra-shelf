import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { withTempRegistry, type RegistrySandbox } from "../helpers";

const E2E = process.env.INFRA_SHELF_E2E === "1";

// Regression test for the re-attach UX bug: after a setup with --envs,
// detaching signoz and re-attaching via plain `add` (no flags) must
// preserve the original env+service.name via AppEntry persistence.
describe.skipIf(!E2E)("signoz detach + reattach preserves env", () => {
  const APP = `e2edetach-${Date.now()}-staging`;
  let sandbox: RegistrySandbox;

  beforeEach(async () => {
    delete process.env.INFRA_SHELF_SECRET;
    sandbox = await withTempRegistry();
  });

  afterEach(async () => {
    try {
      const { removeCommand } = await import(`../../src/commands/remove?cache=${Math.random()}`);
      await removeCommand(APP, true).catch(() => {});
    } catch {}
    await sandbox.cleanup();
  });

  test("re-attach without flags inherits AppEntry.environment", async () => {
    const { setupCommand } = await import(`../../src/commands/setup?cache=${Math.random()}`);
    await setupCommand(APP, ["postgres", "signoz"], { env: "staging" });

    const { loadRegistry } = await import(`../../src/lib/registry?cache=${Math.random()}`);
    const before = (await loadRegistry()).apps[APP];
    expect(before.services.signoz?.environment).toBe("staging");

    const { detachCommand } = await import(`../../src/commands/detach?cache=${Math.random()}`);
    await detachCommand(APP, ["signoz"]);

    const afterDetach = (await loadRegistry()).apps[APP];
    expect(afterDetach.services.signoz).toBeUndefined();
    // AppEntry retains the env so reattach knows what to do
    expect(afterDetach.environment).toBe("staging");

    const { addCommand } = await import(`../../src/commands/add?cache=${Math.random()}`);
    await addCommand(APP, ["signoz"], {}); // no flags — should pull from AppEntry

    const afterReattach = (await loadRegistry()).apps[APP];
    expect(afterReattach.services.signoz?.environment).toBe("staging");
  });
});
