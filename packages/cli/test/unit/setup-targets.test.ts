import { describe, expect, test } from "bun:test";
import { buildSetupTargets } from "../../src/lib/targets";

describe("buildSetupTargets", () => {
  test("no flags → single target, env undefined", () => {
    expect(buildSetupTargets("iara")).toEqual([
      { name: "iara", signozServiceName: "iara", signozEnv: undefined },
    ]);
  });

  test("--env staging → single target, env stamped", () => {
    expect(buildSetupTargets("teste5-staging", { env: "staging" })).toEqual([
      { name: "teste5-staging", signozServiceName: "teste5-staging", signozEnv: "staging" },
    ]);
  });

  test("--envs staging,production → 2 siblings with shared service.name", () => {
    expect(buildSetupTargets("iara", { envs: ["staging", "production"] })).toEqual([
      { name: "iara-staging", signozServiceName: "iara", signozEnv: "staging" },
      { name: "iara-production", signozServiceName: "iara", signozEnv: "production" },
    ]);
  });

  test("--envs single value still expands (no shortcut)", () => {
    expect(buildSetupTargets("iara", { envs: ["staging"] })).toEqual([
      { name: "iara-staging", signozServiceName: "iara", signozEnv: "staging" },
    ]);
  });

  test("empty envs array falls back to default branch", () => {
    expect(buildSetupTargets("iara", { envs: [] })).toEqual([
      { name: "iara", signozServiceName: "iara", signozEnv: undefined },
    ]);
  });
});
