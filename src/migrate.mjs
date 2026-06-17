import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { expandHome, parseToml } from "./config.mjs";
import { askInput } from "./prompt.mjs";

export async function commandMigrate(config, argv) {
  const flags = parseMigrateFlags(argv);
  if (flags.help) {
    printMigrateHelp();
    return;
  }
  const plan = buildMigratePlan(config, flags);
  if (flags.json) {
    console.log(JSON.stringify(plan, null, 2));
  } else {
    printMigratePlan(plan);
  }
  if (flags.dryRun || plan.action === "noop") {
    return;
  }
  if (!flags.yes && !await confirmMigrate(plan)) {
    if (!flags.json) {
      console.log("Cancelled. No files were written.");
    }
    return;
  }
  applyMigratePlan(plan);
  if (!flags.json) {
    console.log(`[ok] migrated ${plan.aiwPath}`);
    console.log(`[ok] backup ${plan.backupPath}`);
  }
}

export function buildMigratePlan(config, flags = {}) {
  const configDir = path.resolve(expandHome(flags.configDir || config.configDir || path.join(os.homedir(), ".config", "aiw")));
  const aiwPath = path.join(configDir, "aiw.toml");
  if (!fs.existsSync(aiwPath)) {
    const error = new Error(`aiw.toml not found: ${aiwPath}`);
    error.exitCode = 3;
    throw error;
  }
  const source = fs.readFileSync(aiwPath, "utf8");
  const parsed = parseToml(source);
  const existing = parsed.workstation || {};
  const hasLegacyWorkstation = hasWorkstationSection(source);
  const shouldWrite = hasLegacyWorkstation || flags.force;
  const nextSource = shouldWrite ? removeWorkstationToml(source) : source;
  const backupPath = `${aiwPath}.${timestamp()}.bak`;

  return {
    action: shouldWrite ? "migrate" : "noop",
    reason: shouldWrite ? migrationReason({ hasLegacyWorkstation, force: flags.force }) : "no legacy workstation config found",
    configDir,
    aiwPath,
    backupPath: shouldWrite ? backupPath : "",
    dryRun: Boolean(flags.dryRun),
    force: Boolean(flags.force),
    workstation: {
      existing: normalizeMaybeWorkstation(existing),
      target: "runtime"
    },
    writes: shouldWrite
      ? [
          { path: backupPath, action: "backup" },
          { path: aiwPath, action: "write" }
        ]
      : [],
    before: source,
    after: nextSource
  };
}

function applyMigratePlan(plan) {
  fs.copyFileSync(plan.aiwPath, plan.backupPath);
  fs.writeFileSync(plan.aiwPath, plan.after);
}

function hasWorkstationSection(source) {
  return /(^|\n)\[workstation\]\n/.test(source);
}

function removeWorkstationToml(source) {
  const match = source.match(/(^|\n)\[workstation\]\n[\s\S]*?(?=\n\[[^\]]+\]\n|$)/);
  if (!match) {
    return source;
  }
  const prefix = source.slice(0, match.index + match[1].length);
  const suffix = source.slice(match.index + match[0].length);
  return `${prefix}${suffix.replace(/^\n+/, "")}`;
}

function parseMigrateFlags(argv) {
  const flags = {};
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case "--help":
      case "-h":
        flags.help = true;
        break;
      case "--config-dir":
        flags.configDir = argv[++index];
        break;
      case "--workstation-mode":
        argv[++index];
        flags.legacyWorkstationOption = true;
        break;
      case "--workstation-implementation":
        argv[++index];
        flags.legacyWorkstationOption = true;
        break;
      case "--dry-run":
        flags.dryRun = true;
        break;
      case "--json":
        flags.json = true;
        break;
      case "--yes":
      case "-y":
        flags.yes = true;
        break;
      case "--force":
      case "-f":
        flags.force = true;
        break;
      default: {
        const error = new Error(`unknown migrate option: ${arg}`);
        error.exitCode = 2;
        throw error;
      }
    }
  }
  if (flags.legacyWorkstationOption) {
    const error = new Error("--workstation-mode and --workstation-implementation are no longer supported; AIW chooses tmux or cmux from the runtime");
    error.exitCode = 2;
    throw error;
  }
  return flags;
}

function normalizeExistingWorkstation(value) {
  return {
    mode: String(value?.mode || "").trim().toLowerCase(),
    implementation: String(value?.implementation || "").trim().toLowerCase()
  };
}

function normalizeMaybeWorkstation(value) {
  const normalized = normalizeExistingWorkstation(value);
  return normalized.mode || normalized.implementation ? normalized : {};
}

function migrationReason({ hasLegacyWorkstation, force }) {
  if (force) {
    return hasLegacyWorkstation ? "forced legacy workstation cleanup" : "forced rewrite without legacy workstation config";
  }
  return "remove legacy workstation config; runtime now chooses tmux or cmux automatically";
}

async function confirmMigrate(plan) {
  if (!process.stdin.isTTY) {
    const error = new Error("aiw migrate requires --yes when running non-interactively");
    error.exitCode = 2;
    throw error;
  }
  const answer = await askInput(`Migrate ${plan.aiwPath} to runtime tmux/cmux selection? Type y to confirm`);
  return answer.toLowerCase() === "y";
}

function printMigratePlan(plan) {
  console.log("AIW migrate plan");
  console.log(`config: ${plan.configDir}`);
  console.log(`target: runtime tmux/cmux`);
  console.log(`action: ${plan.action}`);
  console.log(`reason: ${plan.reason}`);
  for (const write of plan.writes) {
    console.log(`[${write.action}] ${write.path}`);
  }
}

function printMigrateHelp() {
  console.log(`Usage: aiw migrate [options]

Migrate aiw.toml away from legacy [workstation] selection.

Options:
  --config-dir <path>                         Config directory; defaults to the currently loaded AIW config dir
  --dry-run                                   Print the plan without writing files
  --json                                      Print a structured plan
  --yes                                      Apply without prompting
  --force                                    Rewrite even when no legacy [workstation] section exists`);
}

function timestamp() {
  return new Date().toISOString().replace(/[-:]/g, "").replace(/\..+$/, "");
}
