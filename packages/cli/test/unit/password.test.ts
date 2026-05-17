import { describe, expect, test } from "bun:test";
import { generatePassword } from "../../src/lib/password";

describe("generatePassword", () => {
  test("default length is 24", () => {
    expect(generatePassword()).toHaveLength(24);
  });

  test("respects custom length", () => {
    expect(generatePassword(12)).toHaveLength(12);
    expect(generatePassword(40)).toHaveLength(40);
  });

  test("uses base64url charset (no /, +, =)", () => {
    for (let i = 0; i < 20; i++) {
      expect(generatePassword()).toMatch(/^[A-Za-z0-9_-]+$/);
    }
  });

  test("two calls return different values", () => {
    const a = generatePassword();
    const b = generatePassword();
    expect(a).not.toBe(b);
  });
});
