import { $ } from "bun";
import { dockerExec } from "../lib/docker";
import { generatePassword } from "../lib/password";
import type { PostgresConfig } from "../lib/types";
import { validateAppName } from "../lib/validate";

const CONTAINER = "infra-postgres";
const SUPER_USER = "postgres";

export async function provision(appName: string): Promise<PostgresConfig> {
  validateAppName(appName);
  const password = generatePassword();

  await dockerExec(CONTAINER, [
    "psql",
    "-U",
    SUPER_USER,
    "-c",
    `CREATE DATABASE "${appName}";`,
  ]);

  await dockerExec(CONTAINER, [
    "psql",
    "-U",
    SUPER_USER,
    "-c",
    `CREATE USER "${appName}" WITH PASSWORD '${password}';`,
  ]);

  await dockerExec(CONTAINER, [
    "psql",
    "-U",
    SUPER_USER,
    "-c",
    `GRANT ALL PRIVILEGES ON DATABASE "${appName}" TO "${appName}";`,
  ]);

  await dockerExec(CONTAINER, [
    "psql",
    "-U",
    SUPER_USER,
    "-d",
    appName,
    "-c",
    `ALTER SCHEMA public OWNER TO "${appName}";`,
  ]);

  return { database: appName, username: appName, password };
}

export async function backup(appName: string, filePath: string): Promise<void> {
  validateAppName(appName);
  const sql = await dockerExec(CONTAINER, [
    "pg_dump",
    "-U",
    SUPER_USER,
    "--clean",
    "--if-exists",
    appName,
  ]);
  await Bun.write(filePath, sql);
}

export async function restore(appName: string, filePath: string): Promise<void> {
  validateAppName(appName);
  const sql = await Bun.file(filePath).text();
  const proc = Bun.spawn(
    ["docker", "exec", "-i", CONTAINER, "psql", "-U", SUPER_USER, "-d", appName],
    { stdin: "pipe", stdout: "pipe", stderr: "pipe" },
  );
  proc.stdin.write(sql);
  proc.stdin.end();
  const exitCode = await proc.exited;

  if (exitCode !== 0) {
    const stderr = await new Response(proc.stderr).text();
    throw new Error(`psql restore failed: ${stderr.trim()}`);
  }

  await grantPermissions(appName);
}

async function grantPermissions(appName: string): Promise<void> {
  const sql = `
    DO $$ DECLARE r record;
    BEGIN
      FOR r IN SELECT nspname FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname != 'information_schema'
      LOOP
        EXECUTE format('GRANT ALL ON SCHEMA %I TO %I', r.nspname, '${appName}');
        EXECUTE format('GRANT ALL ON ALL TABLES IN SCHEMA %I TO %I', r.nspname, '${appName}');
        EXECUTE format('GRANT ALL ON ALL SEQUENCES IN SCHEMA %I TO %I', r.nspname, '${appName}');
        EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT ALL ON TABLES TO %I', r.nspname, '${appName}');
        EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I GRANT ALL ON SEQUENCES TO %I', r.nspname, '${appName}');
      END LOOP;
    END $$;
  `;
  await dockerExec(CONTAINER, ["psql", "-U", SUPER_USER, "-d", appName, "-c", sql]);
}

export async function teardown(appName: string): Promise<void> {
  validateAppName(appName);
  await dockerExec(CONTAINER, [
    "psql",
    "-U",
    SUPER_USER,
    "-c",
    `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${appName}';`,
  ]);

  await dockerExec(CONTAINER, [
    "psql",
    "-U",
    SUPER_USER,
    "-c",
    `DROP DATABASE IF EXISTS "${appName}";`,
  ]);

  await dockerExec(CONTAINER, [
    "psql",
    "-U",
    SUPER_USER,
    "-c",
    `DROP USER IF EXISTS "${appName}";`,
  ]);
}
