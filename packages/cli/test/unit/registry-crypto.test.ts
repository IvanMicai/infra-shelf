import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import {
  decryptRegistry,
  encryptRegistry,
  isEncryptedRegistryFile,
} from "../../src/lib/registry-crypto";
import type { Registry } from "../../src/lib/types";

const SAMPLE_REGISTRY: Registry = {
  version: 1,
  apps: {
    iara: {
      createdAt: "2026-05-17T00:00:00.000Z",
      services: {
        postgres: { database: "iara", username: "iara", password: "secret" },
      },
    },
  },
};

describe("isEncryptedRegistryFile", () => {
  test("returns true for envelope", () => {
    expect(
      isEncryptedRegistryFile({
        version: 2,
        encrypted: true,
        algorithm: "AES-256-GCM",
        kdf: "SHA-256",
        nonce: "x",
        ciphertext: "y",
      }),
    ).toBe(true);
  });

  test("returns false for plain registry", () => {
    expect(isEncryptedRegistryFile(SAMPLE_REGISTRY)).toBe(false);
  });

  test("returns false for non-objects", () => {
    expect(isEncryptedRegistryFile(null)).toBe(false);
    expect(isEncryptedRegistryFile("string")).toBe(false);
    expect(isEncryptedRegistryFile(42)).toBe(false);
  });
});

describe("encrypt/decrypt round-trip", () => {
  const originalSecret = process.env.INFRA_SHELF_SECRET;

  beforeEach(() => {
    process.env.INFRA_SHELF_SECRET = "test-secret-key";
  });

  afterEach(() => {
    if (originalSecret === undefined) delete process.env.INFRA_SHELF_SECRET;
    else process.env.INFRA_SHELF_SECRET = originalSecret;
  });

  test("encrypted output is structurally sound + decrypts back", async () => {
    const encrypted = await encryptRegistry(SAMPLE_REGISTRY);
    expect(encrypted.encrypted).toBe(true);
    expect(encrypted.algorithm).toBe("AES-256-GCM");
    expect(encrypted.nonce.length).toBeGreaterThan(0);
    expect(encrypted.ciphertext.length).toBeGreaterThan(0);

    const decrypted = await decryptRegistry(encrypted);
    expect(decrypted).toEqual(SAMPLE_REGISTRY);
  });

  test("decrypt fails with wrong secret", async () => {
    const encrypted = await encryptRegistry(SAMPLE_REGISTRY);
    process.env.INFRA_SHELF_SECRET = "different-secret";
    await expect(decryptRegistry(encrypted)).rejects.toThrow();
  });

  test("encrypt fails without secret", async () => {
    delete process.env.INFRA_SHELF_SECRET;
    delete process.env.INFRA_SHELF_REGISTRY_SECRET;
    await expect(encryptRegistry(SAMPLE_REGISTRY)).rejects.toThrow(/INFRA_SHELF_SECRET/);
  });

  test("decrypt rejects truncated payload", async () => {
    const encrypted = await encryptRegistry(SAMPLE_REGISTRY);
    encrypted.ciphertext = encrypted.ciphertext.slice(0, 8);
    await expect(decryptRegistry(encrypted)).rejects.toThrow();
  });
});
