import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export function projectRoot() {
  return ROOT;
}

export function aiwBinPath() {
  return path.join(ROOT, "bin", "aiw");
}

export function expandHome(value) {
  if (typeof value !== "string") {
    return value;
  }
  if (value === "~") {
    return os.homedir();
  }
  if (value.startsWith("~/")) {
    return path.join(os.homedir(), value.slice(2));
  }
  return value;
}

export function loadConfig() {
  const configDir = process.env.AIW_CONFIG_DIR
    ? expandHome(process.env.AIW_CONFIG_DIR)
    : firstExistingDir([
        path.join(os.homedir(), ".config", "aiw"),
        path.join(ROOT, "config")
      ]);

  const aiwPath = path.join(configDir, "aiw.toml");
  const agentsPath = path.join(configDir, "agents.toml");
  const aiw = fs.existsSync(aiwPath) ? parseToml(fs.readFileSync(aiwPath, "utf8")) : {};
  const agents = fs.existsSync(agentsPath) ? parseToml(fs.readFileSync(agentsPath, "utf8")) : {};

  return {
    configDir,
    aiwPath,
    agentsPath,
    defaults: aiw.defaults || {},
    paths: normalizePaths(aiw.paths || {}),
    behavior: aiw.behavior || {},
    commit: aiw.commit || {},
    git: aiw.git || {},
    workspace: aiw.workspace || {},
    agents: agents.agents || {}
  };
}

export function resolveAgent(config, requestedAgent) {
  const name = requestedAgent || config.defaults.agent || "codex";
  const entry = config.agents[name];
  if (!entry || typeof entry.cmd !== "string" || entry.cmd.length === 0) {
    const error = new Error(`unknown agent '${name}' in ${config.agentsPath}`);
    error.exitCode = 2;
    throw error;
  }
  return {
    name,
    cmd: entry.cmd,
    args: Array.isArray(entry.args) ? entry.args.map(String) : [],
    commitArgs: Array.isArray(entry.commit_args) ? entry.commit_args.map(String) : []
  };
}

function normalizePaths(paths) {
  return {
    code_root: expandHome(paths.code_root || path.join(os.homedir(), "Code")),
    worktrees: expandHome(paths.worktrees || "~/worktrees"),
    core_config: expandHome(paths.core_config || ROOT)
  };
}

function firstExistingDir(candidates) {
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }
  return candidates[candidates.length - 1];
}

export function parseToml(source) {
  const root = {};
  let current = root;
  for (const rawLine of source.split(/\r?\n/)) {
    const line = stripComment(rawLine).trim();
    if (!line) {
      continue;
    }
    const sectionMatch = line.match(/^\[([A-Za-z0-9_.-]+)\]$/);
    if (sectionMatch) {
      current = root;
      for (const part of sectionMatch[1].split(".")) {
        current[part] = current[part] || {};
        current = current[part];
      }
      continue;
    }
    const eq = line.indexOf("=");
    if (eq === -1) {
      continue;
    }
    const key = line.slice(0, eq).trim();
    const value = line.slice(eq + 1).trim();
    current[key] = parseTomlValue(value);
  }
  return root;
}

function stripComment(line) {
  let inString = false;
  let escaped = false;
  for (let index = 0; index < line.length; index += 1) {
    const char = line[index];
    if (escaped) {
      escaped = false;
      continue;
    }
    if (char === "\\") {
      escaped = true;
      continue;
    }
    if (char === '"') {
      inString = !inString;
      continue;
    }
    if (char === "#" && !inString) {
      return line.slice(0, index);
    }
  }
  return line;
}

function parseTomlValue(value) {
  if (value === "true") {
    return true;
  }
  if (value === "false") {
    return false;
  }
  if (/^-?\d+(\.\d+)?$/.test(value)) {
    return Number(value);
  }
  if (value.startsWith('"') && value.endsWith('"')) {
    return value.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, "\\");
  }
  if (value.startsWith("[") && value.endsWith("]")) {
    const body = value.slice(1, -1).trim();
    if (!body) {
      return [];
    }
    return splitArray(body).map((item) => parseTomlValue(item.trim()));
  }
  return value;
}

function splitArray(body) {
  const items = [];
  let start = 0;
  let inString = false;
  let escaped = false;
  for (let index = 0; index < body.length; index += 1) {
    const char = body[index];
    if (escaped) {
      escaped = false;
      continue;
    }
    if (char === "\\") {
      escaped = true;
      continue;
    }
    if (char === '"') {
      inString = !inString;
      continue;
    }
    if (char === "," && !inString) {
      items.push(body.slice(start, index));
      start = index + 1;
    }
  }
  items.push(body.slice(start));
  return items;
}
