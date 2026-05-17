// Funções puras que decidem QUAIS apps `setup`/`add` vão tocar dado o
// nome de entrada + flags. Sem I/O — recebem o registry (quando precisa)
// como argumento. Permite testar a lógica de expansão de envs sem
// provisionar containers.

import type { AppEntry, Registry } from "./types";

export interface Target {
  name: string;
  signozServiceName: string;
  signozEnv?: string;
}

export interface BuildOptions {
  envs?: string[];
  env?: string;
}

// Setup: cria apps novos. Sem registry fallback (não existe estado prévio).
export function buildSetupTargets(appName: string, options?: BuildOptions): Target[] {
  if (options?.envs && options.envs.length > 0) {
    return options.envs.map((env) => ({
      name: `${appName}-${env}`,
      signozServiceName: appName,
      signozEnv: env,
    }));
  }
  return [
    {
      name: appName,
      signozServiceName: appName,
      signozEnv: options?.env,
    },
  ];
}

// Add: anexa em apps existentes. Quando nem --envs nem --env vêm,
// puxa do AppEntry persistido (environment + signozServiceName).
// Assim, re-attach via UI preserva o env do setup original.
export function buildAddTargets(
  appName: string,
  options: BuildOptions | undefined,
  registry: Registry,
): Target[] {
  if (options?.envs && options.envs.length > 0) {
    return options.envs.map((env) => ({
      name: `${appName}-${env}`,
      signozServiceName: appName,
      signozEnv: env,
    }));
  }

  const entry: AppEntry | undefined = registry.apps[appName];
  return [
    {
      name: appName,
      signozServiceName: entry?.signozServiceName ?? appName,
      signozEnv: options?.env ?? entry?.environment,
    },
  ];
}
