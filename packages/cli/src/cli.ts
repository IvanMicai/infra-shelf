import { parseArgs } from "node:util";
import type { ServiceName } from "./lib/types";
import { log } from "./lib/output";

const VALID_SERVICES = new Set(["postgres", "redis", "rabbitmq", "aistor", "signoz"]);

function printUsage(): void {
  console.log(`
  Usage: bun shelf <command> [options]

  Commands:
    setup <app>    Provision resources for an app
    add <app>      Attach more services to an existing app
    list           List all provisioned apps
    remove <app>   Remove resources for an app
    backup <app>   Backup app data
    restore <app>  Restore app data from backup
    status         Show infrastructure container status
    registry       Registry maintenance

  Examples:
    bun shelf setup my-app -s postgres,redis,rabbitmq,aistor,signoz
    bun shelf setup my-app -s redis --full-access
    bun shelf add my-app -s aistor,signoz
    bun shelf list --json
    bun shelf remove my-app --force
    bun shelf backup my-app
    bun shelf backup --all
    bun shelf restore my-app
    bun shelf restore my-app --file backups/my-app/postgres_20260404T163000.sql
    bun shelf status
    INFRA_SHELF_SECRET=... bun shelf registry encrypt
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
        "full-access": { type: "boolean", default: false },
      },
    });

    const appName = positionals[0];
    const serviceList = values.services?.split(",") ?? [];
    const fullAccess = values["full-access"] ?? false;

    const invalid = serviceList.filter((s) => !VALID_SERVICES.has(s));
    if (invalid.length > 0) {
      log.error(`Invalid services: ${invalid.join(", ")}. Valid: postgres, redis, rabbitmq, aistor, signoz`);
      process.exit(1);
    }

    const { setupCommand } = await import("./commands/setup");
    await setupCommand(appName, serviceList as ServiceName[], { fullAccess });
    break;
  }

  case "add": {
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
      log.error(`Invalid services: ${invalid.join(", ")}. Valid: postgres, redis, rabbitmq, aistor, signoz`);
      process.exit(1);
    }

    const { addCommand } = await import("./commands/add");
    await addCommand(appName, serviceList as ServiceName[]);
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

  case "backup": {
    const { values, positionals } = parseArgs({
      args: commandArgs,
      allowPositionals: true,
      options: {
        services: { type: "string", short: "s" },
        all: { type: "boolean", short: "a", default: false },
      },
    });

    const serviceList = values.services?.split(",") ?? [];
    const invalid = serviceList.filter((s) => s && !VALID_SERVICES.has(s));
    if (invalid.length > 0) {
      log.error(`Invalid services: ${invalid.join(", ")}. Valid: postgres, redis, rabbitmq, aistor, signoz`);
      process.exit(1);
    }

    const { backupCommand } = await import("./commands/backup");
    await backupCommand(
      positionals[0],
      values.all ?? false,
      serviceList.length ? (serviceList as ServiceName[]) : undefined,
    );
    break;
  }

  case "restore": {
    const { values, positionals } = parseArgs({
      args: commandArgs,
      allowPositionals: true,
      options: {
        services: { type: "string", short: "s" },
        file: { type: "string" },
        force: { type: "boolean", short: "f", default: false },
      },
    });

    const serviceList = values.services?.split(",") ?? [];
    const invalid = serviceList.filter((s) => s && !VALID_SERVICES.has(s));
    if (invalid.length > 0) {
      log.error(`Invalid services: ${invalid.join(", ")}. Valid: postgres, redis, rabbitmq, aistor, signoz`);
      process.exit(1);
    }

    const { restoreCommand } = await import("./commands/restore");
    await restoreCommand(
      positionals[0],
      serviceList.length ? (serviceList as ServiceName[]) : undefined,
      values.file,
      values.force ?? false,
    );
    break;
  }

  case "status": {
    const { statusCommand } = await import("./commands/status");
    await statusCommand();
    break;
  }

  case "registry": {
    const subcommand = commandArgs[0];
    if (subcommand !== "encrypt") {
      log.error("Unknown registry command. Use: bun shelf registry encrypt");
      process.exit(1);
    }

    const { encryptRegistryFile } = await import("./lib/registry");
    try {
      await encryptRegistryFile();
      log.success("Registry encrypted.");
    } catch (err) {
      log.error(err instanceof Error ? err.message : String(err));
      process.exit(1);
    }
    break;
  }

  default:
    printUsage();
    if (command && command !== "--help" && command !== "-h") {
      log.error(`Unknown command: ${command}`);
      process.exit(1);
    }
}
