import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { Database } from "bun:sqlite";
import type { Registry, AppEntry } from "../src/lib/types";

// Wraps the unit-of-work in a temp dir + env override so tests don't
// trample the dev's real apps.json. Caller must invoke `cleanup()` in
// `afterEach`, ideally via try/finally.
export interface RegistrySandbox {
  dir: string;
  path: string;
  cleanup: () => Promise<void>;
}

export async function withTempRegistry(initial?: Registry): Promise<RegistrySandbox> {
  const dir = await mkdtemp(join(tmpdir(), "infra-shelf-registry-"));
  const path = join(dir, "apps.json");

  if (initial) {
    await writeFile(path, JSON.stringify(initial, null, 2), "utf8");
  }

  const previous = process.env.INFRA_SHELF_REGISTRY_PATH;
  process.env.INFRA_SHELF_REGISTRY_PATH = path;

  return {
    dir,
    path,
    cleanup: async () => {
      if (previous === undefined) {
        delete process.env.INFRA_SHELF_REGISTRY_PATH;
      } else {
        process.env.INFRA_SHELF_REGISTRY_PATH = previous;
      }
      await rm(dir, { recursive: true, force: true });
    },
  };
}

export function seedRegistry(apps: Record<string, AppEntry>): Registry {
  return { version: 1, apps };
}

// Schedule store mirrors the Go scheduler.Store schema (see
// packages/app/internal/scheduler/store.go:80-122). We replicate just
// the columns the CLI touches; the Go side handles full migration in
// prod.
export interface ScheduleSandbox {
  dir: string;
  path: string;
  cleanup: () => Promise<void>;
}

export async function withTempScheduleDb(): Promise<ScheduleSandbox> {
  const dir = await mkdtemp(join(tmpdir(), "infra-shelf-schedule-"));
  const path = join(dir, "schedule.db");

  const db = new Database(path);
  db.exec(`
    CREATE TABLE schedules (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      app_name TEXT NOT NULL,
      services TEXT NOT NULL DEFAULT '',
      cron_expr TEXT NOT NULL,
      timezone TEXT NOT NULL,
      retention_days INTEGER NOT NULL DEFAULT 30,
      retention_count INTEGER NOT NULL DEFAULT 0,
      enabled INTEGER NOT NULL DEFAULT 1,
      created_at TEXT NOT NULL,
      updated_at TEXT NOT NULL,
      last_run_at TEXT,
      next_run_at TEXT,
      last_status TEXT NOT NULL DEFAULT '',
      last_message TEXT NOT NULL DEFAULT ''
    );
    CREATE TABLE backup_runs (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      schedule_id INTEGER,
      app_name TEXT NOT NULL,
      services TEXT NOT NULL DEFAULT '',
      status TEXT NOT NULL,
      output TEXT NOT NULL DEFAULT '',
      started_at TEXT NOT NULL,
      finished_at TEXT
    );
  `);
  db.close();

  const previous = process.env.APP_DATABASE_PATH;
  process.env.APP_DATABASE_PATH = path;

  return {
    dir,
    path,
    cleanup: async () => {
      if (previous === undefined) {
        delete process.env.APP_DATABASE_PATH;
      } else {
        process.env.APP_DATABASE_PATH = previous;
      }
      await rm(dir, { recursive: true, force: true });
    },
  };
}

// Captures both stdout and stderr writes for the duration of `fn`.
// Bun's test runner doesn't have a built-in equivalent of jest.spyOn so
// we monkey-patch console.{log,error,warn} directly.
export interface CaptureResult {
  stdout: string;
  stderr: string;
}

export async function captureStdout<T>(fn: () => Promise<T> | T): Promise<CaptureResult & { result: T }> {
  const stdoutBuf: string[] = [];
  const stderrBuf: string[] = [];

  const origLog = console.log;
  const origInfo = console.info;
  const origWarn = console.warn;
  const origError = console.error;

  console.log = (...args: unknown[]) => stdoutBuf.push(args.map(String).join(" "));
  console.info = (...args: unknown[]) => stdoutBuf.push(args.map(String).join(" "));
  console.warn = (...args: unknown[]) => stdoutBuf.push(args.map(String).join(" "));
  console.error = (...args: unknown[]) => stderrBuf.push(args.map(String).join(" "));

  try {
    const result = await fn();
    return {
      result,
      stdout: stdoutBuf.join("\n"),
      stderr: stderrBuf.join("\n"),
    };
  } finally {
    console.log = origLog;
    console.info = origInfo;
    console.warn = origWarn;
    console.error = origError;
  }
}

// process.exit doesn't throw — it just kills the process. To test
// commands that call exit, we replace it with a thrower for the duration
// of the call.
export class ExitError extends Error {
  constructor(public code: number) {
    super(`process.exit(${code})`);
  }
}

export async function withMockedExit<T>(fn: () => Promise<T> | T): Promise<T | { exitCode: number }> {
  const orig = process.exit;
  process.exit = ((code?: number) => {
    throw new ExitError(code ?? 0);
  }) as never;
  try {
    return await fn();
  } catch (err) {
    if (err instanceof ExitError) {
      return { exitCode: err.code };
    }
    throw err;
  } finally {
    process.exit = orig;
  }
}
