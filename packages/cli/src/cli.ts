import { parseArgs } from "node:util";
import type { ServiceName } from "./lib/types";
import { log } from "./lib/output";

const VALID_SERVICES = new Set(["postgres", "redis", "rabbitmq"]);

function printUsage(): void {
  console.log(`
  Usage: bun shelf <command> [options]

  Commands:
    setup <app>   Provision resources for an app
    list          List all provisioned apps
    remove <app>  Remove resources for an app
    status        Show infrastructure container status

  Examples:
    bun shelf setup my-app -s postgres,redis,rabbitmq
    bun shelf list --json
    bun shelf remove my-app --force
    bun shelf status
`);
}

const args = process.argv.slice(2);
const command = args[0];
const commandArgs = args.slice(1);

switch (command) {
  case "setup": {
    const { values, positionals } = parseArgs({
      args: commandArgs,
      allowPositionals: true,
      options: {
        services: { type: "string", short: "s" },
      },
    });

    const appName = positionals[0];
    const serviceList = values.services?.split(",") ?? [];

    const invalid = serviceList.filter((s) => !VALID_SERVICES.has(s));
    if (invalid.length > 0) {
      log.error(`Invalid services: ${invalid.join(", ")}. Valid: postgres, redis, rabbitmq`);
      process.exit(1);
    }

    const { setupCommand } = await import("./commands/setup");
    await setupCommand(appName, serviceList as ServiceName[]);
    break;
  }

  case "list": {
    const { values } = parseArgs({
      args: commandArgs,
      options: {
        json: { type: "boolean", short: "j", default: false },
      },
    });

    const { listCommand } = await import("./commands/list");
    await listCommand(values.json ?? false);
    break;
  }

  case "remove": {
    const { values, positionals } = parseArgs({
      args: commandArgs,
      allowPositionals: true,
      options: {
        force: { type: "boolean", short: "f", default: false },
      },
    });

    const { removeCommand } = await import("./commands/remove");
    await removeCommand(positionals[0], values.force ?? false);
    break;
  }

  case "status": {
    const { statusCommand } = await import("./commands/status");
    await statusCommand();
    break;
  }

  default:
    printUsage();
    if (command && command !== "--help" && command !== "-h") {
      log.error(`Unknown command: ${command}`);
      process.exit(1);
    }
}
