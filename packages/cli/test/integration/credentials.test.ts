import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import {
  withTempRegistry,
  captureStdout,
  withMockedExit,
  type RegistrySandbox,
} from "../helpers";
import type { Registry } from "../../src/lib/types";

async function freshCommand() {
  return await import(`../../src/commands/credentials?cache=${Math.random()}`);
}

async function writeRegistry(path: string, registry: Registry): Promise<void> {
  await Bun.write(path, JSON.stringify(registry, null, 2));
}

describe("credentialsCommand", () => {
  let sandbox: RegistrySandbox;
  const origSecret = process.env.INFRA_SHELF_SECRET;

  beforeEach(async () => {
    delete process.env.INFRA_SHELF_SECRET;
    sandbox = await withTempRegistry();
  });

  afterEach(async () => {
    await sandbox.cleanup();
    if (origSecret !== undefined) process.env.INFRA_SHELF_SECRET = origSecret;
  });

  test("nonexistent app exits with error", async () => {
    await writeRegistry(sandbox.path, { version: 1, apps: {} });
    const mod = await freshCommand();

    const captured = await captureStdout(() =>
      withMockedExit(() => mod.credentialsCommand("ghost")),
    );

    expect(captured.stderr).toContain('App "ghost" not found');
    expect(captured.result).toEqual({ exitCode: 1 });
  });

  test("app with full set of services prints all blocks", async () => {
    await writeRegistry(sandbox.path, {
      version: 1,
      apps: {
        iara: {
          createdAt: "2026-05-17T00:00:00.000Z",
          services: {
            postgres: { database: "iara", username: "iara", password: "p" },
            redis: { username: "iara", password: "rp", prefix: "iara:" },
            rabbitmq: { vhost: "iara", username: "iara", password: "qp" },
            aistor: {
              bucket: "iara",
              accessKey: "ak",
              secretKey: "sk",
              endpoint: "http://aistor:9000",
            },
            signoz: { serviceName: "iara", environment: "dev" },
          },
        },
      },
    });

    const mod = await freshCommand();
    const captured = await captureStdout(() => mod.credentialsCommand("iara"));

    expect(captured.stdout).toContain("# === PostgreSQL ===");
    expect(captured.stdout).toContain("# === Redis ===");
    expect(captured.stdout).toContain("# === RabbitMQ ===");
    expect(captured.stdout).toContain("# === AIStor (S3) ===");
    expect(captured.stdout).toContain("# === SignOz (OpenTelemetry) ===");
    expect(captured.stdout).toContain("OTEL_SERVICE_NAME=iara");
  });

  test("app without services warns", async () => {
    await writeRegistry(sandbox.path, {
      version: 1,
      apps: { empty: { createdAt: "x", services: {} } },
    });

    const mod = await freshCommand();
    const captured = await captureStdout(() => mod.credentialsCommand("empty"));

    expect(captured.stdout).toContain("no services attached");
  });
});
