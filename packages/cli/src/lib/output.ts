import type {
  PostgresConfig,
  RedisConfig,
  RabbitmqConfig,
  AistorConfig,
  SignozConfig,
} from "./types";

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
  const lines = [
    `# === Redis ===`,
    `REDIS_URL=${url}`,
    `REDIS_HOST=redis`,
    `REDIS_PORT=6379`,
    `REDIS_USERNAME=${config.username}`,
    `REDIS_PASSWORD=${config.password}`,
  ];
  if (config.prefix) {
    lines.push(`REDIS_PREFIX=${config.prefix}`);
  }
  return lines.join("\n");
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

export function aistorEnv(config: AistorConfig): string {
  return [
    `# === AIStor (S3) ===`,
    `S3_ENDPOINT=${config.endpoint}`,
    `S3_BUCKET=${config.bucket}`,
    `S3_REGION=us-east-1`,
    `S3_ACCESS_KEY_ID=${config.accessKey}`,
    `S3_SECRET_ACCESS_KEY=${config.secretKey}`,
    `S3_FORCE_PATH_STYLE=true`,
    `AWS_ENDPOINT_URL=${config.endpoint}`,
    `AWS_ACCESS_KEY_ID=${config.accessKey}`,
    `AWS_SECRET_ACCESS_KEY=${config.secretKey}`,
    `AWS_REGION=us-east-1`,
  ].join("\n");
}

export function signozEnv(config: SignozConfig): string {
  const attrs = `service.name=${config.serviceName},service.namespace=infra-shelf,deployment.environment=${config.environment}`;
  return [
    `# === SignOz (OpenTelemetry) ===`,
    `OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317`,
    `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`,
    `OTEL_SERVICE_NAME=${config.serviceName}`,
    `OTEL_RESOURCE_ATTRIBUTES=${attrs}`,
    `OTEL_TRACES_EXPORTER=otlp`,
    `OTEL_METRICS_EXPORTER=otlp`,
    `OTEL_LOGS_EXPORTER=otlp`,
  ].join("\n");
}
