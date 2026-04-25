import { resolve, dirname } from "node:path";
import type { Registry } from "./types";
import {
  decryptRegistry,
  encryptRegistry,
  getRegistrySecret,
  isEncryptedRegistryFile,
} from "./registry-crypto";

const DEFAULT_REGISTRY_PATH = resolve(dirname(import.meta.dir), "apps.json");
const REGISTRY_PATH = process.env.INFRA_SHELF_REGISTRY_PATH
  ? resolve(process.env.INFRA_SHELF_REGISTRY_PATH)
  : DEFAULT_REGISTRY_PATH;

function createEmpty(): Registry {
  return {
    version: 1,
    apps: {},
  };
}

export async function loadRegistry(): Promise<Registry> {
  const file = Bun.file(REGISTRY_PATH);
  if (!(await file.exists())) {
    return createEmpty();
  }

  const content = await file.json();
  if (isEncryptedRegistryFile(content)) {
    return decryptRegistry(content);
  }
  return content as Registry;
}

export async function saveRegistry(registry: Registry): Promise<void> {
  if (await getRegistrySecret()) {
    const encrypted = await encryptRegistry(registry);
    await Bun.write(REGISTRY_PATH, JSON.stringify(encrypted, null, 2) + "\n");
    return;
  }

  await Bun.write(REGISTRY_PATH, JSON.stringify(registry, null, 2) + "\n");
}

export async function encryptRegistryFile(): Promise<void> {
  const registry = await loadRegistry();
  const encrypted = await encryptRegistry(registry);
  await Bun.write(REGISTRY_PATH, JSON.stringify(encrypted, null, 2) + "\n");
}
