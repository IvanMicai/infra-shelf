import { resolve } from "node:path";
import { dockerExec } from "../lib/docker";
import { generatePassword } from "../lib/password";
import type { RedisConfig } from "../lib/types";
import { validateAppName } from "../lib/validate";

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
  validateAppName(appName);
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

  await adminCli(["ACL", "SAVE"]);

  return { username: appName, password, prefix };
}

const BACKUP_SCRIPT = `
local keys = redis.call('KEYS', ARGV[1])
local result = {}
for i, key in ipairs(keys) do
  local t = redis.call('TYPE', key)['ok']
  local val
  if t == 'string' then val = redis.call('GET', key)
  elseif t == 'hash' then val = redis.call('HGETALL', key)
  elseif t == 'list' then val = redis.call('LRANGE', key, 0, -1)
  elseif t == 'set' then val = redis.call('SMEMBERS', key)
  elseif t == 'zset' then val = redis.call('ZRANGE', key, 0, -1, 'WITHSCORES')
  end
  result[#result+1] = {key, t, val}
end
return cjson.encode(result)
`.trim();

export async function backup(appName: string, filePath: string): Promise<void> {
  validateAppName(appName);
  const json = await adminCli(["EVAL", BACKUP_SCRIPT, "0", `${appName}:*`]);
  await Bun.write(filePath, json);
}

export async function restore(appName: string, filePath: string): Promise<void> {
  validateAppName(appName);
  const raw = await Bun.file(filePath).text();
  const data = JSON.parse(raw) as [string, string, unknown][];

  for (const [key, type, value] of data) {
    switch (type) {
      case "string":
        await adminCli(["SET", key, value as string]);
        break;
      case "hash":
        await adminCli(["HSET", key, ...(value as string[])]);
        break;
      case "list":
        await adminCli(["RPUSH", key, ...(value as string[])]);
        break;
      case "set":
        await adminCli(["SADD", key, ...(value as string[])]);
        break;
      case "zset": {
        const pairs = value as string[];
        const args: string[] = [];
        for (let i = 0; i < pairs.length; i += 2) {
          args.push(pairs[i + 1], pairs[i]); // ZADD expects: score member
        }
        if (args.length > 0) {
          await adminCli(["ZADD", key, ...args]);
        }
        break;
      }
    }
  }
}

export async function teardown(appName: string): Promise<void> {
  validateAppName(appName);
  // Delete keys with app prefix
  await adminCli([
    "EVAL",
    "local keys = redis.call('keys', ARGV[1]) for i=1,#keys do redis.call('del', keys[i]) end return #keys",
    "0",
    `${appName}:*`,
  ]);

  // Remove the ACL user
  await adminCli(["ACL", "DELUSER", appName]);
  await adminCli(["ACL", "SAVE"]);
}
