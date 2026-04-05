import type { PostgresConfig, RedisConfig, RabbitmqConfig } from "./types";

const green = (s: string) => `\x1b[32m${s}\x1b[0m`;
const cyan = (s: string) => `\x1b[36m${s}\x1b[0m`;
const red = (s: string) => `\x1b[31m${s}\x1b[0m`;
const bold = (s: string) => `\x1b[1m${s}\x1b[0m`;
const dim = (s: string) => `\x1b[2m${s}\x1b[0m`;

export const log = {
  success: (msg: string) => console.log(green(`✔ ${msg}`)),
  error: (msg: string) => console.error(red(`✘ ${msg}`)),
  info: (msg: string) => console.log(cyan(`ℹ ${msg}`)),
  warn: (msg: string) => console.log(`⚠ ${msg}`),
  title: (msg: string) => console.log(bold(msg)),
  dim: (msg: string) => console.log(dim(msg)),
};

export function postgresEnv(config: PostgresConfig): string {
  const url = `postgres://${config.username}:${config.password}@postgres:5432/${config.database}`;
  return [
    `# === PostgreSQL ===`,
    `DATABASE_URL=${url}`,
    `DB_HOST=postgres`,
    `DB_PORT=5432`,
    `DB_USERNAME=${config.username}`,
    `DB_PASSWORD=${config.password}`,
    `DB_NAME=${config.database}`,
  ].join("\n");
}

export function redisEnv(config: RedisConfig): string {
  const url = `redis://${config.username}:${config.password}@redis:6379/0`;
  return [
    `# === Redis ===`,
    `REDIS_URL=${url}`,
    `REDIS_HOST=redis`,
    `REDIS_PORT=6379`,
    `REDIS_USERNAME=${config.username}`,
    `REDIS_PASSWORD=${config.password}`,
    `REDIS_PREFIX=${config.prefix}`,
  ].join("\n");
}

export function rabbitmqEnv(config: RabbitmqConfig): string {
  const vhost = encodeURIComponent(config.vhost);
  const url = `amqp://${config.username}:${config.password}@rabbitmq:5672/${vhost}`;
  return [
    `# === RabbitMQ ===`,
    `AMQP_URL=${url}`,
    `RABBITMQ_HOST=rabbitmq`,
    `RABBITMQ_PORT=5672`,
    `RABBITMQ_USERNAME=${config.username}`,
    `RABBITMQ_PASSWORD=${config.password}`,
    `RABBITMQ_VHOST=${config.vhost}`,
  ].join("\n");
}
