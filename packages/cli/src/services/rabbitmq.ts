import { dockerExec } from "../lib/docker";
import { generatePassword } from "../lib/password";
import type { RabbitmqConfig } from "../lib/types";

const CONTAINER = "infra-rabbitmq";

export async function provision(appName: string): Promise<RabbitmqConfig> {
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

export async function teardown(appName: string): Promise<void> {
  await dockerExec(CONTAINER, ["rabbitmqctl", "delete_user", appName]);
  await dockerExec(CONTAINER, ["rabbitmqctl", "delete_vhost", appName]);
}
