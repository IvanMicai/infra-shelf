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
