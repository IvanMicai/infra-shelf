import { resolve } from "node:path";
import { dockerExec } from "../lib/docker";
import { generatePassword } from "../lib/password";
import type { RedisConfig } from "../lib/types";

const CONTAINER = "infra-redis";

async function getAdminPassword(): Promise<string> {
  const envPath = resolve(process.cwd(), ".env");
  const file = Bun.file(envPath);
  if (!(await file.exists())) return "";
  const content = await file.text();
  const match = content.match(/^REDIS_PASSWORD=(.*)$/m);
  return match?.[1]?.trim() ?? "";
}

async function adminCli(args: string[]): Promise<string> {
  const adminPass = await getAdminPassword();
  const authArgs = adminPass ? ["-a", adminPass] : [];
  return dockerExec(CONTAINER, ["redis-cli", ...authArgs, ...args]);
}

export async function provision(appName: string): Promise<RedisConfig> {
  const password = generatePassword();
  const prefix = `${appName}:`;

  await adminCli([
    "ACL",
    "SETUSER",
    appName,
    "on",
    `>${password}`,
    `~${prefix}*`,
    "+@all",
  ]);

  return { username: appName, password, prefix };
}

export async function teardown(appName: string): Promise<void> {
  // Delete keys with app prefix
  await adminCli([
    "EVAL",
    "local keys = redis.call('keys', ARGV[1]) for i=1,#keys do redis.call('del', keys[i]) end return #keys",
    "0",
    `${appName}:*`,
  ]);

  // Remove the ACL user
  await adminCli(["ACL", "DELUSER", appName]);
}
