import { $ } from "bun";

export class DockerExecError extends Error {
  constructor(
    public container: string,
    public command: string[],
    public exitCode: number,
    public stderr: string,
  ) {
    super(`docker exec ${container} failed (exit ${exitCode}): ${stderr}`);
  }
}

export async function dockerExec(
  container: string,
  command: string[],
): Promise<string> {
  const result = await $`docker exec ${container} ${command}`
    .nothrow()
    .quiet();

  if (result.exitCode !== 0) {
    throw new DockerExecError(
      container,
      command,
      result.exitCode,
      result.stderr.toString().trim(),
    );
  }

  return result.stdout.toString().trim();
}

export async function isContainerRunning(container: string): Promise<boolean> {
  try {
    const result =
      await $`docker inspect --format={{.State.Running}} ${container}`
        .nothrow()
        .quiet();
    return result.stdout.toString().trim() === "true";
  } catch {
    return false;
  }
}
