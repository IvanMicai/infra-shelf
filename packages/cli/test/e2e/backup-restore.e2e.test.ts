import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { readdir, rm } from "node:fs/promises";
import { resolve } from "node:path";
import { withTempRegistry, type RegistrySandbox } from "../helpers";
import { dockerExec } from "../../src/lib/docker";

const E2E = process.env.INFRA_SHELF_E2E === "1";
const BACKUPS_DIR = resolve(process.cwd(), "backups");

describe.skipIf(!E2E)("postgres backup → restore preserves data", () => {
  const APP = `e2ebkp${Date.now()}`;
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
    await rm(resolve(BACKUPS_DIR, APP), { recursive: true, force: true });
    await sandbox.cleanup();
  });

  test("dump + restore round-trip", async () => {
    const { setupCommand } = await import(`../../src/commands/setup?cache=${Math.random()}`);
    await setupCommand(APP, ["postgres"]);

    // Seed a known row so we can detect data preservation
    await dockerExec("infra-postgres", [
      "psql",
      "-U",
      "postgres",
      "-d",
      APP,
      "-c",
      "CREATE TABLE smoke (id int); INSERT INTO smoke VALUES (42);",
    ]);

    const { backupCommand } = await import(`../../src/commands/backup?cache=${Math.random()}`);
    await backupCommand(APP, false, ["postgres"]);

    // Wipe the table so restore has work to do
    await dockerExec("infra-postgres", [
      "psql",
      "-U",
      "postgres",
      "-d",
      APP,
      "-c",
      "DROP TABLE smoke;",
    ]);

    const files = (await readdir(resolve(BACKUPS_DIR, APP))).filter((f) =>
      f.startsWith("postgres_"),
    );
    expect(files.length).toBeGreaterThan(0);

    const { restoreCommand } = await import(`../../src/commands/restore?cache=${Math.random()}`);
    await restoreCommand(APP, ["postgres"], resolve(BACKUPS_DIR, APP, files[0]!), true);

    const result = await dockerExec("infra-postgres", [
      "psql",
      "-U",
      "postgres",
      "-d",
      APP,
      "-tAc",
      "SELECT id FROM smoke;",
    ]);
    expect(result.trim()).toBe("42");
  });
});
