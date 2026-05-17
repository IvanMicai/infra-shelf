import { resolve, basename } from "node:path";
import { unlink, stat } from "node:fs/promises";
import { log } from "../lib/output";

const BACKUPS_DIR = resolve(process.cwd(), "backups");

// Removes a single backup file from the local backups directory. S3 mirror
// cleanup isn't done here — the web app's UploadAll cycle reconciles the
// remote on next sync. Use `bun shelf backup delete <app> <file>`.
export async function backupDeleteCommand(
  appName: string,
  fileName: string,
): Promise<void> {
  if (!appName || !fileName) {
    log.error("Usage: bun shelf backup delete <app> <file>");
    process.exit(1);
  }
  if (basename(fileName) !== fileName) {
    log.error("Refusing to traverse paths — pass just the file name.");
    process.exit(1);
  }

  const path = resolve(BACKUPS_DIR, appName, fileName);
  try {
    await stat(path);
  } catch {
    log.error(`Not found: ${path}`);
    process.exit(1);
  }

  await unlink(path);
  log.success(`removed ${appName}/${fileName}`);
}
