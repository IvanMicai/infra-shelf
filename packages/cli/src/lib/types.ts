export type ServiceName = "postgres" | "redis" | "rabbitmq";

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

export interface AppEntry {
  createdAt: string;
  services: {
    postgres?: PostgresConfig;
    redis?: RedisConfig;
    rabbitmq?: RabbitmqConfig;
  };
}

export interface Registry {
  version: number;
  apps: Record<string, AppEntry>;
}
