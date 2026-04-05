import { $ } from "bun";
import { log } from "../lib/output";

const CONTAINERS = [
  { name: "infra-postgres", service: "PostgreSQL" },
  { name: "infra-redis", service: "Redis" },
  { name: "infra-rabbitmq", service: "RabbitMQ" },
];

export async function statusCommand(): Promise<void> {
  log.title("Infrastructure Status\n");

  for (const { name, service } of CONTAINERS) {
    try {
      const result =
        await $`docker inspect --format={{.State.Status}} ${name}`
          .nothrow()
          .quiet();

      const status = result.stdout.toString().trim();

      if (result.exitCode !== 0) {
        console.log(`  ${service.padEnd(14)} ⏹  not created`);
      } else if (status === "running") {
        console.log(`  ${service.padEnd(14)} 🟢 running`);
      } else {
        console.log(`  ${service.padEnd(14)} 🔴 ${status}`);
      }
    } catch {
      console.log(`  ${service.padEnd(14)} ⏹  not found`);
    }
  }

  console.log("");
}
