import { describe, expect, test } from "bun:test";
import {
  postgresEnv,
  redisEnv,
  rabbitmqEnv,
  aistorEnv,
  signozEnv,
} from "../../src/lib/output";

describe("env builders", () => {
  test("postgresEnv renders DATABASE_URL + components", () => {
    const out = postgresEnv({ database: "foo", username: "foo", password: "pw" });
    expect(out).toContain("# === PostgreSQL ===");
    expect(out).toContain("DATABASE_URL=postgres://foo:pw@postgres:5432/foo");
    expect(out).toContain("DB_HOST=postgres");
    expect(out).toContain("DB_NAME=foo");
  });

  test("redisEnv includes prefix when set", () => {
    const out = redisEnv({ username: "foo", password: "pw", prefix: "foo:" });
    expect(out).toContain("REDIS_URL=redis://foo:pw@redis:6379/0");
    expect(out).toContain("REDIS_PREFIX=foo:");
  });

  test("redisEnv omits prefix when empty", () => {
    const out = redisEnv({ username: "foo", password: "pw", prefix: "" });
    expect(out).not.toContain("REDIS_PREFIX");
  });

  test("rabbitmqEnv url-encodes vhost with special chars", () => {
    const out = rabbitmqEnv({ vhost: "foo/bar", username: "u", password: "p" });
    // encodeURIComponent("/") === "%2F"
    expect(out).toContain("amqp://u:p@rabbitmq:5672/foo%2Fbar");
    expect(out).toContain("RABBITMQ_VHOST=foo/bar");
  });

  test("aistorEnv exposes both S3_* and AWS_* aliases", () => {
    const out = aistorEnv({
      bucket: "foo",
      accessKey: "k",
      secretKey: "s",
      endpoint: "http://aistor:9000",
    });
    expect(out).toContain("S3_ENDPOINT=http://aistor:9000");
    expect(out).toContain("AWS_ENDPOINT_URL=http://aistor:9000");
    expect(out).toContain("AWS_ACCESS_KEY_ID=k");
  });

  test("signozEnv composes resource attributes", () => {
    const out = signozEnv({ serviceName: "iara", environment: "staging" });
    expect(out).toContain("OTEL_SERVICE_NAME=iara");
    expect(out).toContain(
      "OTEL_RESOURCE_ATTRIBUTES=service.name=iara,service.namespace=infra-shelf,deployment.environment=staging",
    );
    expect(out).toContain("OTEL_EXPORTER_OTLP_ENDPOINT=http://signoz-otel-collector:4317");
  });
});
