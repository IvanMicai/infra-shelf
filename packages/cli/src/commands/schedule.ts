import { Database } from "bun:sqlite";
import { resolve } from "node:path";
import { mkdir } from "node:fs/promises";
import { log } from "../lib/output";

// Mirrors packages/app/internal/scheduler/store.go schema. Reads the same
// SQLite file the web UI uses (APP_DATABASE_PATH), so CLI and UI share
// state. The cron tick and retention application live in the web app's
// scheduler.Manager — CLI changes are picked up on the next reload.
const DEFAULT_DB_PATH = resolve(process.cwd(), "data/app/infra-shelf-app.db");

interface ScheduleRow {
  id: number;
  app_name: string;
  services: string;
  cron_expr: string;
  timezone: string;
  retention_days: number;
  retention_count: number;
  enabled: number;
  created_at: string;
  updated_at: string;
  last_run_at: string | null;
  next_run_at: string | null;
  last_status: string;
  last_message: string;
}

async function openDb(): Promise<Database> {
  const path = process.env.APP_DATABASE_PATH ?? DEFAULT_DB_PATH;
  await mkdir(resolve(path, ".."), { recursive: true });
  const db = new Database(path);
  db.exec("PRAGMA busy_timeout = 5000; PRAGMA foreign_keys = ON;");
  // Migrations are owned by the web app's Go scheduler.Store. We trust
  // them to have run already (the web app touches this DB on boot).
  return db;
}

function nowRfc3339(): string {
  return new Date().toISOString().replace(/\.\d+Z$/, "Z");
}

export async function scheduleListCommand(): Promise<void> {
  const db = await openDb();
  try {
    const rows = db
      .query("SELECT * FROM schedules ORDER BY enabled DESC, id DESC")
      .all() as ScheduleRow[];
    if (rows.length === 0) {
      log.info("No schedules.");
      return;
    }
    for (const r of rows) {
      const state = r.enabled ? "enabled" : "paused";
      const services = r.services || "(all)";
      const last = r.last_run_at ? `last=${r.last_run_at} (${r.last_status})` : "never run";
      const next = r.next_run_at ? `next=${r.next_run_at}` : "no upcoming";
      console.log(`#${r.id} ${r.app_name} ${state}`);
      console.log(`  cron="${r.cron_expr}" tz=${r.timezone} services=${services}`);
      console.log(`  retention: ${r.retention_days}d / ${r.retention_count} files`);
      console.log(`  ${last}  |  ${next}`);
    }
  } finally {
    db.close();
  }
}

export interface ScheduleCreateOptions {
  appName: string;
  cronExpr: string;
  timezone?: string;
  services?: string[];
  retentionDays?: number;
  retentionCount?: number;
  enabled?: boolean;
}

export async function scheduleCreateCommand(opts: ScheduleCreateOptions): Promise<void> {
  if (!opts.appName) {
    log.error("App name is required.");
    process.exit(1);
  }
  if (!opts.cronExpr) {
    log.error("--cron is required.");
    process.exit(1);
  }

  const db = await openDb();
  try {
    const now = nowRfc3339();
    const result = db.run(
      `INSERT INTO schedules (app_name, services, cron_expr, timezone, retention_days, retention_count, enabled, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [
        opts.appName,
        (opts.services ?? []).join(","),
        opts.cronExpr,
        opts.timezone ?? "America/Sao_Paulo",
        opts.retentionDays ?? 30,
        opts.retentionCount ?? 0,
        opts.enabled === false ? 0 : 1,
        now,
        now,
      ],
    );
    log.success(`schedule #${result.lastInsertRowid} created`);
    log.info("Restart the web app (or wait for next manager reload) to pick it up.");
  } finally {
    db.close();
  }
}

async function setEnabled(id: number, enabled: boolean): Promise<void> {
  const db = await openDb();
  try {
    const result = db.run(
      `UPDATE schedules SET enabled = ?, updated_at = ? WHERE id = ?`,
      [enabled ? 1 : 0, nowRfc3339(), id],
    );
    if (result.changes === 0) {
      log.error(`schedule #${id} not found`);
      process.exit(1);
    }
    log.success(`schedule #${id} ${enabled ? "resumed" : "paused"}`);
  } finally {
    db.close();
  }
}

export async function schedulePauseCommand(id: number): Promise<void> {
  await setEnabled(id, false);
}

export async function scheduleResumeCommand(id: number): Promise<void> {
  await setEnabled(id, true);
}

export async function scheduleDeleteCommand(id: number): Promise<void> {
  const db = await openDb();
  try {
    const result = db.run(`DELETE FROM schedules WHERE id = ?`, [id]);
    if (result.changes === 0) {
      log.error(`schedule #${id} not found`);
      process.exit(1);
    }
    log.success(`schedule #${id} deleted`);
  } finally {
    db.close();
  }
}
