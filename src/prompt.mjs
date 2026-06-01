import readline from "node:readline/promises";
import { stdin as input, stdout as output } from "node:process";
import { spawnSync } from "node:child_process";
import { commandExists } from "./run.mjs";

export async function askInput(label, options = {}) {
  if (!process.stdin.isTTY && !options.allowNonTty) {
    const error = new Error(`${label} is required, but stdin is not interactive`);
    error.exitCode = 4;
    throw error;
  }
  const rl = readline.createInterface({ input, output });
  try {
    const suffix = options.defaultValue ? ` (${options.defaultValue})` : "";
    const answer = (await rl.question(`${label}${suffix}: `)).trim();
    return answer || options.defaultValue || "";
  } finally {
    rl.close();
  }
}

export async function pickFromList(label, items, options = {}) {
  if (items.length === 1 && !options.force) {
    return items[0];
  }
  const orderedItems = orderDefault(items, options.defaultItem);
  if (process.stdin.isTTY && commandExists("fzf")) {
    const result = spawnSync("fzf", ["--prompt", `${label}> `], {
      input: `${orderedItems.join("\n")}\n`,
      encoding: "utf8",
      stdio: ["pipe", "pipe", "inherit"]
    });
    if (result.status === 0 && result.stdout.trim()) {
      return result.stdout.trim();
    }
  }
  if (!process.stdin.isTTY) {
    const error = new Error(`${label} requires an interactive terminal`);
    error.exitCode = 4;
    throw error;
  }
  orderedItems.forEach((item, index) => {
    console.log(`${index + 1}. ${item}`);
  });
  const answer = await askInput(label);
  const index = Number(answer);
  if (!Number.isInteger(index) || index < 1 || index > orderedItems.length) {
    const error = new Error(`invalid selection: ${answer}`);
    error.exitCode = 4;
    throw error;
  }
  return orderedItems[index - 1];
}

function orderDefault(items, defaultItem) {
  if (!defaultItem || !items.includes(defaultItem)) {
    return items;
  }
  return [defaultItem, ...items.filter((item) => item !== defaultItem)];
}
