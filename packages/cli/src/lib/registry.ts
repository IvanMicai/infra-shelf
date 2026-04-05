import { resolve, dirname } from "node:path";
import type { Registry } from "./types";

const REGISTRY_PATH = resolve(dirname(import.meta.dir), "apps.json");

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
  return (await file.json()) as Registry;
}

export async function saveRegistry(registry: Registry): Promise<void> {
  await Bun.write(REGISTRY_PATH, JSON.stringify(registry, null, 2) + "\n");
}
