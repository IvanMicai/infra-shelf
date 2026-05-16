import { resolve } from "node:path";
import { generatePassword } from "../lib/password";
import type { AistorConfig } from "../lib/types";
import { validateAppName } from "../lib/validate";

const CONTAINER = "infra-aistor";
const ALIAS = "local";
const ENDPOINT_HOST = "aistor";
const ENDPOINT_PORT = 9000;

interface RootCreds {
  user: string;
  pass: string;
}

async function getRootCreds(): Promise<RootCreds> {
  const envPath = resolve(process.cwd(), ".env");
  const file = Bun.file(envPath);
  if (!(await file.exists())) {
    throw new Error(
      "AIStor: .env not found. Set AISTOR_ROOT_USER and AISTOR_ROOT_PASSWORD.",
    );
  }
  const content = await file.text();
  const user = content.match(/^AISTOR_ROOT_USER=(.*)$/m)?.[1]?.trim() ?? "";
  const pass = content.match(/^AISTOR_ROOT_PASSWORD=(.*)$/m)?.[1]?.trim() ?? "";
  if (!user || !pass) {
    throw new Error(
      "AIStor: AISTOR_ROOT_USER and AISTOR_ROOT_PASSWORD must be set in .env.",
    );
  }
  return { user, pass };
}

async function mcHostEnv(): Promise<string> {
  const { user, pass } = await getRootCreds();
  const userEnc = encodeURIComponent(user);
  const passEnc = encodeURIComponent(pass);
  return `MC_HOST_${ALIAS}=http://${userEnc}:${passEnc}@localhost:${ENDPOINT_PORT}`;
}

async function mc(args: string[]): Promise<string> {
  const env = await mcHostEnv();
  const proc = Bun.spawn(
    ["docker", "exec", "-e", env, CONTAINER, "mc", ...args],
    { stdout: "pipe", stderr: "pipe" },
  );
  const exitCode = await proc.exited;
  const stdout = await new Response(proc.stdout).text();
  if (exitCode !== 0) {
    const stderr = (await new Response(proc.stderr).text()).trim();
    throw new Error(`mc ${args.join(" ")} failed: ${stderr || stdout.trim()}`);
  }
  return stdout.trim();
}

async function mcShell(script: string): Promise<string> {
  const env = await mcHostEnv();
  const proc = Bun.spawn(
    ["docker", "exec", "-e", env, CONTAINER, "sh", "-c", script],
    { stdout: "pipe", stderr: "pipe" },
  );
  const exitCode = await proc.exited;
  const stdout = await new Response(proc.stdout).text();
  if (exitCode !== 0) {
    const stderr = (await new Response(proc.stderr).text()).trim();
    throw new Error(`mc shell failed: ${stderr || stdout.trim()}`);
  }
  return stdout.trim();
}

function policyName(appName: string): string {
  return `${appName}-rw`;
}

function bucketPolicy(appName: string): string {
  return JSON.stringify({
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: ["s3:*"],
        Resource: [
          `arn:aws:s3:::${appName}`,
          `arn:aws:s3:::${appName}/*`,
        ],
      },
    ],
  });
}

export async function provision(appName: string): Promise<AistorConfig> {
  validateAppName(appName);
  const secretKey = generatePassword();
  const policy = policyName(appName);

  await mc(["mb", "--ignore-existing", `${ALIAS}/${appName}`]);
  await mc(["admin", "user", "add", ALIAS, appName, secretKey]);

  const policyJson = bucketPolicy(appName).replace(/'/g, `'\\''`);
  await mcShell(
    `printf '%s' '${policyJson}' > /tmp/aistor-policy-${appName}.json && ` +
      `mc admin policy create ${ALIAS} ${policy} /tmp/aistor-policy-${appName}.json && ` +
      `rm -f /tmp/aistor-policy-${appName}.json`,
  );

  await mc(["admin", "policy", "attach", ALIAS, policy, "--user", appName]);

  return {
    bucket: appName,
    accessKey: appName,
    secretKey,
    endpoint: `http://${ENDPOINT_HOST}:${ENDPOINT_PORT}`,
  };
}

export async function backup(appName: string, filePath: string): Promise<void> {
  validateAppName(appName);
  const env = await mcHostEnv();
  const script =
    `set -e; ` +
    `rm -rf /tmp/aistor-bk && mkdir -p /tmp/aistor-bk && ` +
    `mc mirror --quiet --overwrite ${ALIAS}/${appName} /tmp/aistor-bk/${appName}/ >/dev/null && ` +
    `tar -cf - -C /tmp/aistor-bk ${appName}; ` +
    `rc=$?; rm -rf /tmp/aistor-bk; exit $rc`;

  const proc = Bun.spawn(
    ["docker", "exec", "-e", env, CONTAINER, "sh", "-c", script],
    { stdout: "pipe", stderr: "pipe" },
  );
  const tarBytes = await new Response(proc.stdout).arrayBuffer();
  const exitCode = await proc.exited;
  if (exitCode !== 0) {
    const stderr = (await new Response(proc.stderr).text()).trim();
    throw new Error(`aistor backup failed: ${stderr || `exit ${exitCode}`}`);
  }
  await Bun.write(filePath, tarBytes);
}

export async function restore(
  appName: string,
  filePath: string,
): Promise<void> {
  validateAppName(appName);
  const env = await mcHostEnv();
  const tarBytes = await Bun.file(filePath).arrayBuffer();

  const script =
    `set -e; ` +
    `rm -rf /tmp/aistor-rs && mkdir -p /tmp/aistor-rs && ` +
    `tar -xf - -C /tmp/aistor-rs && ` +
    `mc mb --ignore-existing ${ALIAS}/${appName} >/dev/null && ` +
    `mc mirror --quiet --overwrite --remove /tmp/aistor-rs/${appName}/ ${ALIAS}/${appName} >/dev/null; ` +
    `rc=$?; rm -rf /tmp/aistor-rs; exit $rc`;

  const proc = Bun.spawn(
    ["docker", "exec", "-i", "-e", env, CONTAINER, "sh", "-c", script],
    { stdin: "pipe", stdout: "pipe", stderr: "pipe" },
  );
  proc.stdin.write(new Uint8Array(tarBytes));
  proc.stdin.end();
  const exitCode = await proc.exited;
  if (exitCode !== 0) {
    const stderr = (await new Response(proc.stderr).text()).trim();
    throw new Error(`aistor restore failed: ${stderr || `exit ${exitCode}`}`);
  }
}

export async function teardown(appName: string): Promise<void> {
  validateAppName(appName);
  const policy = policyName(appName);

  await safe(() =>
    mc(["rb", "--force", "--dangerous", `${ALIAS}/${appName}`]),
  );
  await safe(() =>
    mc(["admin", "policy", "detach", ALIAS, policy, "--user", appName]),
  );
  await safe(() => mc(["admin", "policy", "rm", ALIAS, policy]));
  await safe(() => mc(["admin", "user", "remove", ALIAS, appName]));
}

async function safe(fn: () => Promise<unknown>): Promise<void> {
  try {
    await fn();
  } catch {
    // best-effort teardown — ignore missing resources
  }
}
