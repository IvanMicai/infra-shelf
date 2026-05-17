import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { withTempRegistry, seedRegistry, type RegistrySandbox } from "../helpers";
import type { Registry } from "../../src/lib/types";

// Re-import per test so the env override in withTempRegistry is picked up
// — the module caches REGISTRY_PATH at load time, so we need a fresh
// module instance for each test. Bun resolves dynamic imports per call.
async function freshRegistryModule() {
  return await import(`../../src/lib/registry?cache=${Math.random()}`);
}

describe("registry plain (no INFRA_SHELF_SECRET)", () => {
  let sandbox: RegistrySandbox;
  const origSecret = process.env.INFRA_SHELF_SECRET;

  beforeEach(async () => {
    delete process.env.INFRA_SHELF_SECRET;
    delete process.env.INFRA_SHELF_REGISTRY_SECRET;
    sandbox = await withTempRegistry();
  });

  afterEach(async () => {
    await sandbox.cleanup();
    if (origSecret !== undefined) process.env.INFRA_SHELF_SECRET = origSecret;
  });

  test("loadRegistry returns empty when file is missing", async () => {
    const mod = await freshRegistryModule();
    const registry = await mod.loadRegistry();
    expect(registry).toEqual({ version: 1, apps: {} });
  });

  test("save then load round-trips", async () => {
    const mod = await freshRegistryModule();
    const input: Registry = seedRegistry({
      foo: {
        createdAt: "2026-05-17T00:00:00.000Z",
        environment: "staging",
        signozServiceName: "foo-base",
        services: {
          postgres: { database: "foo", username: "foo", password: "p" },
        },
      },
    });

    await mod.saveRegistry(input);
    const loaded = await mod.loadRegistry();
    expect(loaded).toEqual(input);
  });
});

describe("registry encrypted (with INFRA_SHELF_SECRET)", () => {
  let sandbox: RegistrySandbox;
  const origSecret = process.env.INFRA_SHELF_SECRET;

  beforeEach(async () => {
    process.env.INFRA_SHELF_SECRET = "test-key-for-registry";
    sandbox = await withTempRegistry();
  });

  afterEach(async () => {
    await sandbox.cleanup();
    if (origSecret === undefined) delete process.env.INFRA_SHELF_SECRET;
    else process.env.INFRA_SHELF_SECRET = origSecret;
  });

  test("save writes encrypted envelope; load decrypts back", async () => {
    const mod = await freshRegistryModule();
    const input: Registry = seedRegistry({
      bar: { createdAt: "2026-05-17T00:00:00.000Z", services: {} },
    });

    await mod.saveRegistry(input);

    // Read raw file to confirm it's encrypted, not plain JSON
    const raw = await Bun.file(sandbox.path).json();
    expect(raw.encrypted).toBe(true);
    expect(raw.algorithm).toBe("AES-256-GCM");

    const loaded = await mod.loadRegistry();
    expect(loaded).toEqual(input);
  });
});
