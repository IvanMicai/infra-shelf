import type { SignozConfig } from "../lib/types";

export async function provision(appName: string): Promise<SignozConfig> {
  return {
    serviceName: appName,
    environment: process.env.SIGNOZ_DEFAULT_ENV ?? "dev",
  };
}

export async function teardown(_appName: string): Promise<void> {
  // SignOz Community has no per-app tenancy or credentials to remove —
  // registry entry deletion alone is sufficient. Historical telemetry stays
  // in ClickHouse and ages out via the SignOz retention policy.
}
