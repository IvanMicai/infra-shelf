import { describe, expect, test } from "bun:test";
import { parseEnvs, parseSingleEnv } from "../../src/lib/envs";

describe("parseEnvs", () => {
  test("undefined input returns undefined", () => {
    expect(parseEnvs(undefined)).toBeUndefined();
  });

  test("single value becomes one-item array", () => {
    expect(parseEnvs("staging")).toEqual(["staging"]);
  });

  test("comma-separated values expand", () => {
    expect(parseEnvs("staging,production")).toEqual(["staging", "production"]);
  });

  test("trims whitespace", () => {
    expect(parseEnvs("  staging , production ")).toEqual(["staging", "production"]);
  });

  test("rejects empty string", () => {
    expect(() => parseEnvs("")).toThrow(/at least one/);
  });

  test("rejects invalid name", () => {
    expect(() => parseEnvs("Staging")).toThrow(/Invalid env name/);
    expect(() => parseEnvs("staging,Prod")).toThrow(/Invalid env name/);
  });

  test("rejects duplicates", () => {
    expect(() => parseEnvs("staging,staging")).toThrow(/Duplicate envs/);
  });
});

describe("parseSingleEnv", () => {
  test("undefined input returns undefined", () => {
    expect(parseSingleEnv(undefined)).toBeUndefined();
  });

  test("empty string returns undefined", () => {
    expect(parseSingleEnv("")).toBeUndefined();
    expect(parseSingleEnv("   ")).toBeUndefined();
  });

  test("valid single value passes through", () => {
    expect(parseSingleEnv("staging")).toBe("staging");
  });

  test("trims whitespace", () => {
    expect(parseSingleEnv("  staging  ")).toBe("staging");
  });

  test("rejects invalid name", () => {
    expect(() => parseSingleEnv("Staging")).toThrow(/Invalid env name/);
    expect(() => parseSingleEnv("staging,production")).toThrow(/Invalid env name/);
  });
});
