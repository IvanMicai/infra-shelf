import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { Database } from "bun:sqlite";
import {
  withTempScheduleDb,
  captureStdout,
  withMockedExit,
  type ScheduleSandbox,
} from "../helpers";

async function freshCommand() {
  return await import(`../../src/commands/schedule?cache=${Math.random()}`);
}

describe("schedule CRUD", () => {
  let sandbox: ScheduleSandbox;

  beforeEach(async () => {
    sandbox = await withTempScheduleDb();
  });

  afterEach(async () => {
    await sandbox.cleanup();
  });

  test("list returns 'No schedules' on empty db", async () => {
    const mod = await freshCommand();
    const captured = await captureStdout(() => mod.scheduleListCommand());
    expect(captured.stdout).toContain("No schedules");
  });

  test("create → list shows the new schedule", async () => {
    const mod = await freshCommand();
    await mod.scheduleCreateCommand({
      appName: "iara",
      cronExpr: "0 3 * * *",
      services: ["postgres"],
      retentionDays: 7,
    });

    const captured = await captureStdout(() => mod.scheduleListCommand());
    expect(captured.stdout).toContain("iara");
    expect(captured.stdout).toContain("enabled");
    expect(captured.stdout).toContain('cron="0 3 * * *"');
    expect(captured.stdout).toContain("services=postgres");
    expect(captured.stdout).toContain("retention: 7d");
  });

  test("create applies defaults for timezone, retention", async () => {
    const mod = await freshCommand();
    await mod.scheduleCreateCommand({
      appName: "minimal",
      cronExpr: "@daily",
    });

    const db = new Database(sandbox.path);
    const row = db.query("SELECT * FROM schedules WHERE app_name = ?").get("minimal") as Record<string, unknown>;
    db.close();

    expect(row.timezone).toBe("America/Sao_Paulo");
    expect(row.retention_days).toBe(30);
    expect(row.retention_count).toBe(0);
    expect(row.enabled).toBe(1);
  });

  test("pause → resume cycle flips enabled flag", async () => {
    const mod = await freshCommand();
    await mod.scheduleCreateCommand({ appName: "iara", cronExpr: "@hourly" });

    const db = new Database(sandbox.path);
    const id = (db.query("SELECT id FROM schedules LIMIT 1").get() as { id: number }).id;
    db.close();

    await mod.schedulePauseCommand(id);
    const dbA = new Database(sandbox.path);
    expect((dbA.query("SELECT enabled FROM schedules WHERE id=?").get(id) as { enabled: number }).enabled).toBe(0);
    dbA.close();

    await mod.scheduleResumeCommand(id);
    const dbB = new Database(sandbox.path);
    expect((dbB.query("SELECT enabled FROM schedules WHERE id=?").get(id) as { enabled: number }).enabled).toBe(1);
    dbB.close();
  });

  test("delete removes the row", async () => {
    const mod = await freshCommand();
    await mod.scheduleCreateCommand({ appName: "iara", cronExpr: "@hourly" });

    const db = new Database(sandbox.path);
    const id = (db.query("SELECT id FROM schedules LIMIT 1").get() as { id: number }).id;
    db.close();

    await mod.scheduleDeleteCommand(id);

    const dbAfter = new Database(sandbox.path);
    const row = dbAfter.query("SELECT * FROM schedules").get();
    dbAfter.close();
    expect(row).toBeNull();
  });

  test("delete on nonexistent id exits with error", async () => {
    const mod = await freshCommand();
    const captured = await captureStdout(() =>
      withMockedExit(() => mod.scheduleDeleteCommand(99999)),
    );
    expect(captured.stderr).toContain("not found");
    expect(captured.result).toEqual({ exitCode: 1 });
  });
});
