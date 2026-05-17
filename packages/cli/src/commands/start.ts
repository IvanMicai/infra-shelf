import { spawn } from "node:child_process";
import { log } from "../lib/output";

// Mirrors the web UI's "Start infrastructure" button: a thin wrapper over
// `docker compose up -d`. The web app could shell this out, but having it
// here lets the CLI bootstrap a host without `make` (e.g. TrueNAS via
// `docker run oven/bun`).
export async function startCommand(extra?: string[]): Promise<void> {
  const args = ["compose", "--env-file", ".env", "up", "-d", ...(extra ?? [])];
  log.info(`docker ${args.join(" ")}`);

  await new Promise<void>((resolve, reject) => {
    const proc = spawn("docker", args, { stdio: "inherit" });
    proc.on("error", reject);
    proc.on("exit", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`docker compose exited with code ${code}`));
    });
  });
  log.success("infrastructure started");
}
