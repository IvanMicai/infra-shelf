import { $ } from "bun";
import { dockerExec } from "../lib/docker";
import { generatePassword } from "../lib/password";
import type { RabbitmqConfig } from "../lib/types";
import { validateAppName } from "../lib/validate";

const CONTAINER = "infra-rabbitmq";

export async function provision(appName: string): Promise<RabbitmqConfig> {
  validateAppName(appName);
  const password = generatePassword();

  await dockerExec(CONTAINER, ["rabbitmqctl", "add_vhost", appName]);

  await dockerExec(CONTAINER, [
    "rabbitmqctl",
    "add_user",
    appName,
    password,
  ]);

  await dockerExec(CONTAINER, [
    "rabbitmqctl",
    "set_permissions",
    "-p",
    appName,
    appName,
    ".*",
    ".*",
    ".*",
  ]);

  await dockerExec(CONTAINER, [
    "rabbitmqctl",
    "set_user_tags",
    appName,
    "management",
  ]);

  return { vhost: appName, username: appName, password };
}

function filterByVhost(defs: Record<string, unknown>, vhost: string): Record<string, unknown> {
  const filterArr = (arr: unknown[], key: string) =>
    Array.isArray(arr) ? arr.filter((item: any) => item[key] === vhost) : [];

  return {
    rabbit_version: defs.rabbit_version,
    vhosts: filterArr(defs.vhosts as unknown[], "name"),
    users: filterArr(defs.users as unknown[], "name"),
    permissions: filterArr(defs.permissions as unknown[], "vhost"),
    queues: filterArr(defs.queues as unknown[], "vhost"),
    exchanges: filterArr(defs.exchanges as unknown[], "vhost"),
    bindings: filterArr(defs.bindings as unknown[], "vhost"),
    policies: filterArr(defs.policies as unknown[], "vhost"),
  };
}

export async function backup(appName: string, filePath: string): Promise<void> {
  validateAppName(appName);
  const allDefs = await dockerExec(CONTAINER, [
    "rabbitmqctl",
    "export_definitions",
    "-",
  ]);
  const defs = JSON.parse(allDefs);
  const filtered = filterByVhost(defs, appName);
  await Bun.write(filePath, JSON.stringify(filtered, null, 2));
}

export async function restore(_appName: string, filePath: string): Promise<void> {
  const json = await Bun.file(filePath).text();
  const proc = Bun.spawn(
    ["docker", "exec", "-i", CONTAINER, "rabbitmqctl", "import_definitions", "/dev/stdin"],
    { stdin: "pipe", stdout: "pipe", stderr: "pipe" },
  );
  proc.stdin.write(json);
  proc.stdin.end();
  const exitCode = await proc.exited;

  if (exitCode !== 0) {
    const stderr = await new Response(proc.stderr).text();
    throw new Error(`rabbitmqctl import failed: ${stderr.trim()}`);
  }
}

export async function teardown(appName: string): Promise<void> {
  validateAppName(appName);
  await dockerExec(CONTAINER, ["rabbitmqctl", "delete_user", appName]);
  await dockerExec(CONTAINER, ["rabbitmqctl", "delete_vhost", appName]);
}
