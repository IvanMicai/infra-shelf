export type ServiceName = "postgres" | "redis" | "rabbitmq" | "aistor" | "signoz";

export interface PostgresConfig {
  database: string;
  username: string;
  password: string;
}

export interface RedisConfig {
  username: string;
  password: string;
  prefix: string;
}

export interface RabbitmqConfig {
  vhost: string;
  username: string;
  password: string;
}

export interface AistorConfig {
  bucket: string;
  accessKey: string;
  secretKey: string;
  endpoint: string;
}

export interface SignozConfig {
  serviceName: string;
  environment: string;
}

export interface AppEntry {
  createdAt: string;
  // Stored at setup time when --envs / --env is used. Survives addon
  // detach/reattach so the SignOz block stays consistent across cycles.
  // Optional — apps created without env flags don't have it.
  environment?: string;
  // SignOz service.name override. When --envs creates siblings
  // `<base>-<env>`, all of them carry serviceName=<base> so they group
  // under the same service in the UI, with the env as filter.
  signozServiceName?: string;
  services: {
    postgres?: PostgresConfig;
    redis?: RedisConfig;
    rabbitmq?: RabbitmqConfig;
    aistor?: AistorConfig;
    signoz?: SignozConfig;
  };
}

export interface Registry {
  version: number;
  apps: Record<string, AppEntry>;
}
