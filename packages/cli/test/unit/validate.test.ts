import { describe, expect, test } from "bun:test";
import { validateAppName } from "../../src/lib/validate";

describe("validateAppName", () => {
  test.each([
    "app",
    "my-app",
    "a",
    "iara-staging",
    "test123",
    "test-1-2-3",
  ])("accepts %p", (name) => {
    expect(() => validateAppName(name)).not.toThrow();
  });

  test.each([
    ["", "empty"],
    ["MyApp", "uppercase"],
    ["my_app", "underscore"],
    ["1app", "starts with digit"],
    ["-app", "starts with hyphen"],
    [" app", "leading space"],
    ["my app", "contains space"],
    ["app/foo", "contains slash"],
  ])("rejects %p (%s)", (name) => {
    expect(() => validateAppName(name)).toThrow(/Invalid app name/);
  });
});
