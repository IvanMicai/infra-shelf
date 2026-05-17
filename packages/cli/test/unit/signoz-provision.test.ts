import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { provision } from "../../src/services/signoz";

describe("signoz.provision", () => {
  const originalEnv = process.env.SIGNOZ_DEFAULT_ENV;

  afterEach(() => {
    if (originalEnv === undefined) delete process.env.SIGNOZ_DEFAULT_ENV;
    else process.env.SIGNOZ_DEFAULT_ENV = originalEnv;
  });

  test("default environment is 'dev'", async () => {
    delete process.env.SIGNOZ_DEFAULT_ENV;
    const cfg = await provision("iara");
    expect(cfg).toEqual({ serviceName: "iara", environment: "dev" });
  });

  test("respects SIGNOZ_DEFAULT_ENV", async () => {
    process.env.SIGNOZ_DEFAULT_ENV = "production";
    const cfg = await provision("iara");
    expect(cfg.environment).toBe("production");
  });

  test("options.environment overrides env var", async () => {
    process.env.SIGNOZ_DEFAULT_ENV = "production";
    const cfg = await provision("iara", { environment: "staging" });
    expect(cfg.environment).toBe("staging");
  });

  test("options.serviceName overrides appName", async () => {
    const cfg = await provision("iara-staging", { serviceName: "iara", environment: "staging" });
    expect(cfg.serviceName).toBe("iara");
    expect(cfg.environment).toBe("staging");
  });
});
