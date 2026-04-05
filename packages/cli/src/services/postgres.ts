import { dockerExec } from "../lib/docker";
import { generatePassword } from "../lib/password";
import type { PostgresConfig } from "../lib/types";

const CONTAINER = "infra-postgres";
const SUPER_USER = "postgres";

export async function provision(appName: string): Promise<PostgresConfig> {
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

export async function teardown(appName: string): Promise<void> {
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
