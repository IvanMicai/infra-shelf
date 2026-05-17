import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { Database } from "bun:sqlite";
import { resolve } from "node:path";

const E2E = process.env.INFRA_SHELF_E2E === "1";

// Confirms that `bun shelf schedule create` writes to the same SQLite the
// web app reads from. The DB lives at the configured APP_DATABASE_PATH;
// on a default `make up` it's `./data/app/infra-shelf-app.db`.
describe.skipIf(!E2E)("CLI schedule shares SQLite with web app", () => {
  let createdId: number | null = null;
  const DB_PATH = process.env.APP_DATABASE_PATH ?? resolve(process.cwd(), "data/app/infra-shelf-app.db");
  const APP = `e2esched-${Date.now()}`;

  afterEach(() => {
    if (createdId !== null) {
      const db = new Database(DB_PATH);
      db.run("DELETE FROM schedules WHERE id = ?", [createdId]);
      db.close();
      createdId = null;
    }
  });

  test("schedule create from CLI is visible via raw SQLite query", async () => {
    const { scheduleCreateCommand } = await import(
      `../../src/commands/schedule?cache=${Math.random()}`
    );
    await scheduleCreateCommand({
      appName: APP,
      cronExpr: "0 4 * * *",
      services: ["postgres"],
      retentionDays: 14,
    });

    const db = new Database(DB_PATH);
    const row = db
      .query("SELECT id, app_name, cron_expr, retention_days, enabled FROM schedules WHERE app_name = ?")
      .get(APP) as { id: number; app_name: string; cron_expr: string; retention_days: number; enabled: number } | null;
    db.close();

    expect(row).not.toBeNull();
    expect(row!.app_name).toBe(APP);
    expect(row!.cron_expr).toBe("0 4 * * *");
    expect(row!.retention_days).toBe(14);
    expect(row!.enabled).toBe(1);

    createdId = row!.id;
  });
});
