import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { mkdir, mkdtemp, rm, writeFile, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { captureStdout, withMockedExit } from "../helpers";

async function freshCommand() {
  return await import(`../../src/commands/backup-delete?cache=${Math.random()}`);
}

describe("backupDeleteCommand", () => {
  let workDir: string;
  let prevCwd: string;

  beforeEach(async () => {
    workDir = await mkdtemp(join(tmpdir(), "infra-shelf-bd-"));
    prevCwd = process.cwd();
    process.chdir(workDir);
    await mkdir(join(workDir, "backups", "iara"), { recursive: true });
  });

  afterEach(async () => {
    process.chdir(prevCwd);
    await rm(workDir, { recursive: true, force: true });
  });

  test("removes an existing file", async () => {
    const file = join(workDir, "backups", "iara", "postgres_20260517T100000.sql");
    await writeFile(file, "-- dump\n");

    const mod = await freshCommand();
    const captured = await captureStdout(() =>
      mod.backupDeleteCommand("iara", "postgres_20260517T100000.sql"),
    );

    expect(captured.stdout).toContain("removed iara/postgres_20260517T100000.sql");
    await expect(stat(file)).rejects.toThrow();
  });

  test("nonexistent file exits with error", async () => {
    const mod = await freshCommand();
    const captured = await captureStdout(() =>
      withMockedExit(() => mod.backupDeleteCommand("iara", "missing.sql")),
    );

    expect(captured.stderr).toContain("Not found");
    expect(captured.result).toEqual({ exitCode: 1 });
  });

  test("rejects path traversal in file name", async () => {
    const mod = await freshCommand();
    const captured = await captureStdout(() =>
      withMockedExit(() => mod.backupDeleteCommand("iara", "../../etc/passwd")),
    );

    expect(captured.stderr).toContain("Refusing to traverse");
    expect(captured.result).toEqual({ exitCode: 1 });
  });
});
