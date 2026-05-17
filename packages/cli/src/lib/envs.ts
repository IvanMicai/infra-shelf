// Parsers compartilhados pelos flags `--env` (singular, tag-only) e
// `--envs` (plural, expande em siblings). `throw` em erro pra que o
// caller decida entre converter em `process.exit` (CLI) ou propagar
// (testes / chamadores embarcados).

export const ENV_NAME_REGEX = /^[a-z][a-z0-9-]*$/;

export function parseEnvs(raw: string | undefined): string[] | undefined {
  if (raw === undefined) return undefined;
  const parts = raw.split(",").map((s) => s.trim()).filter(Boolean);
  if (parts.length === 0) {
    throw new Error("--envs requires at least one environment name.");
  }
  for (const env of parts) {
    if (!ENV_NAME_REGEX.test(env)) {
      throw new Error(
        `Invalid env name "${env}". Use lowercase letters, numbers, and hyphens.`,
      );
    }
  }
  if (new Set(parts).size !== parts.length) {
    throw new Error(`Duplicate envs in --envs: ${parts.join(",")}`);
  }
  return parts;
}

export function parseSingleEnv(raw: string | undefined): string | undefined {
  if (raw === undefined) return undefined;
  const env = raw.trim();
  if (!env) return undefined;
  if (!ENV_NAME_REGEX.test(env)) {
    throw new Error(
      `Invalid env name "${env}". Use lowercase letters, numbers, and hyphens.`,
    );
  }
  return env;
}
