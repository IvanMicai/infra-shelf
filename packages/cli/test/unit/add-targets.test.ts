import { describe, expect, test } from "bun:test";
import { buildAddTargets } from "../../src/lib/targets";
import type { Registry } from "../../src/lib/types";

function makeRegistry(apps: Registry["apps"] = {}): Registry {
  return { version: 1, apps };
}

describe("buildAddTargets", () => {
  test("no flags + no AppEntry persistence → default single target", () => {
    const registry = makeRegistry({
      iara: { createdAt: "x", services: {} },
    });
    expect(buildAddTargets("iara", undefined, registry)).toEqual([
      { name: "iara", signozServiceName: "iara", signozEnv: undefined },
    ]);
  });

  test("falls back to AppEntry.environment when flags omitted", () => {
    const registry = makeRegistry({
      "iara-staging": {
        createdAt: "x",
        environment: "staging",
        signozServiceName: "iara",
        services: {},
      },
    });
    expect(buildAddTargets("iara-staging", undefined, registry)).toEqual([
      { name: "iara-staging", signozServiceName: "iara", signozEnv: "staging" },
    ]);
  });

  test("--env overrides persisted AppEntry.environment", () => {
    const registry = makeRegistry({
      "iara-staging": {
        createdAt: "x",
        environment: "staging",
        signozServiceName: "iara",
        services: {},
      },
    });
    expect(buildAddTargets("iara-staging", { env: "production" }, registry)).toEqual([
      { name: "iara-staging", signozServiceName: "iara", signozEnv: "production" },
    ]);
  });

  test("--envs ignores persisted state and expands", () => {
    const registry = makeRegistry({
      "iara-staging": { createdAt: "x", services: {} },
      "iara-production": { createdAt: "x", services: {} },
    });
    expect(buildAddTargets("iara", { envs: ["staging", "production"] }, registry)).toEqual([
      { name: "iara-staging", signozServiceName: "iara", signozEnv: "staging" },
      { name: "iara-production", signozServiceName: "iara", signozEnv: "production" },
    ]);
  });

  test("missing entry → signozServiceName falls back to appName", () => {
    const registry = makeRegistry();
    expect(buildAddTargets("ghost", undefined, registry)).toEqual([
      { name: "ghost", signozServiceName: "ghost", signozEnv: undefined },
    ]);
  });
});
