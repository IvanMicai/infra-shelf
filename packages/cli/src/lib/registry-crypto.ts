import { createCipheriv, createDecipheriv, createHash, randomBytes } from "node:crypto";
import { dirname, resolve } from "node:path";
import { readFile } from "node:fs/promises";
import type { Registry } from "./types";

const ALGORITHM = "AES-256-GCM";
const KDF = "SHA-256";
const SECRET_KEYS = ["INFRA_SHELF_SECRET", "INFRA_SHELF_REGISTRY_SECRET"];

interface EncryptedRegistryFile {
  version: 2;
  encrypted: true;
  algorithm: typeof ALGORITHM;
  kdf: typeof KDF;
  nonce: string;
  ciphertext: string;
}

export function isEncryptedRegistryFile(value: unknown): value is EncryptedRegistryFile {
  return Boolean(
    value &&
      typeof value === "object" &&
      "encrypted" in value &&
      (value as { encrypted?: unknown }).encrypted === true,
  );
}

export async function getRegistrySecret(): Promise<string | undefined> {
  for (const key of SECRET_KEYS) {
    const value = process.env[key]?.trim();
    if (value) return value;
  }

  const env = await loadDotenv(process.cwd());
  for (const key of SECRET_KEYS) {
    const value = env[key]?.trim();
    if (value) return value;
  }

  return undefined;
}

export async function encryptRegistry(registry: Registry): Promise<EncryptedRegistryFile> {
  const secret = await getRegistrySecret();
  if (!secret) {
    throw new Error("INFRA_SHELF_SECRET is required to encrypt the registry.");
  }

  const key = deriveKey(secret);
  const nonce = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", key, nonce);
  const plaintext = Buffer.from(JSON.stringify(registry), "utf8");
  const encrypted = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const tag = cipher.getAuthTag();

  return {
    version: 2,
    encrypted: true,
    algorithm: ALGORITHM,
    kdf: KDF,
    nonce: nonce.toString("base64"),
    ciphertext: Buffer.concat([encrypted, tag]).toString("base64"),
  };
}

export async function decryptRegistry(file: EncryptedRegistryFile): Promise<Registry> {
  if (file.algorithm !== ALGORITHM || file.kdf !== KDF) {
    throw new Error(`Unsupported encrypted registry format: ${file.algorithm}/${file.kdf}`);
  }

  const secret = await getRegistrySecret();
  if (!secret) {
    throw new Error("INFRA_SHELF_SECRET is required to read the encrypted registry.");
  }

  const payload = Buffer.from(file.ciphertext, "base64");
  if (payload.length <= 16) {
    throw new Error("Invalid encrypted registry payload.");
  }

  const encrypted = payload.subarray(0, payload.length - 16);
  const tag = payload.subarray(payload.length - 16);
  const decipher = createDecipheriv("aes-256-gcm", deriveKey(secret), Buffer.from(file.nonce, "base64"));
  decipher.setAuthTag(tag);

  const plaintext = Buffer.concat([decipher.update(encrypted), decipher.final()]);
  return JSON.parse(plaintext.toString("utf8")) as Registry;
}

function deriveKey(secret: string): Buffer {
  return createHash("sha256").update(secret, "utf8").digest();
}

async function loadDotenv(startDir: string): Promise<Record<string, string>> {
  let dir = resolve(startDir);

  while (true) {
    try {
      const content = await readFile(resolve(dir, ".env"), "utf8");
      return parseDotenv(content);
    } catch {
      const parent = dirname(dir);
      if (parent === dir) return {};
      dir = parent;
    }
  }
}

function parseDotenv(content: string): Record<string, string> {
  const env: Record<string, string> = {};
  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;
    const eq = line.indexOf("=");
    if (eq === -1) continue;

    const key = line.slice(0, eq).trim();
    let value = line.slice(eq + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    env[key] = value;
  }
  return env;
}
