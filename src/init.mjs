import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { expandHome, projectRoot, resolveAgent } from "./config.mjs";
import { gate } from "./deps.mjs";
import { pickFromList } from "./prompt.mjs";
import { commandPath, tryCapture } from "./run.mjs";

const AIW_CONFIG_FILES = [
  "aiw.toml",
  "agents.toml",
  "commit-prompt.md",
  "lazygit-delta.yml"
];

const AIW_ACTION_IDS = [
  "aiw-new-worktree",
  "aiw-pick-directory",
  "aiw-local-workspace",
  "aiw-scratch-session"
];

const DEFAULT_CONTEXT_MENU = [
  { action: "aiw-new-worktree", title: "AIW New Worktree" },
  { action: "aiw-pick-directory", title: "AIW Pick Directory" },
  { action: "aiw-local-workspace", title: "AIW Local Workspace" },
  { action: "aiw-scratch-session", title: "AIW Scratch Session" },
  { type: "separator" },
  { action: "cmux.newTerminal", title: "New Terminal" },
  { action: "cmux.newBrowser", title: "New Browser" }
];

const INSTALL_HINTS = {
  git: "macOS: xcode-select --install or brew install git; Linux: use your distro package manager.",
  cmux: "Install cmux and make sure the cmux CLI is on PATH.",
  wt: "Install Worktrunk and make sure the wt CLI is on PATH.",
  yazi: "macOS: brew install yazi; Linux: use your distro package manager.",
  nvim: "macOS: brew install neovim; Linux: use your distro package manager.",
  lazygit: "macOS: brew install lazygit; Linux: use your distro package manager.",
  rg: "macOS: brew install ripgrep; Linux: install ripgrep.",
  fzf: "macOS: brew install fzf; Linux: install fzf.",
  bat: "macOS: brew install bat; Linux: install bat.",
  delta: "macOS: brew install git-delta; Linux: install git-delta.",
  "cmux-git-diff": "Install cmux-git-diff or install delta as the supported fallback.",
  node: "Install Node.js >= 18 and ensure node is on PATH.",
  npx: "Install npm/npx with Node.js and ensure npx is on PATH."
};

export async function commandInit(config, argv) {
  const flags = parseInitFlags(argv);
  if (flags.help) {
    printInitHelp();
    return;
  }

  const targetConfigDir = resolveConfigDir(flags.configDir);
  const codeRoot = path.resolve(expandHome(flags.codeRoot || path.join(os.homedir(), "Code")));
  const worktreesRoot = path.resolve(expandHome(flags.worktreesRoot || path.join(os.homedir(), "worktrees")));
  const sessionsRoot = path.resolve(expandHome(flags.sessionsRoot || path.join(os.homedir(), "Documents", "aiw")));
  const cmuxScope = await selectCmuxScope(flags, codeRoot);
  const launcher = flags.launcher || process.env.AIW_INIT_COMMAND || "npx aiw";
  const plan = buildInitPlan({
    config,
    flags,
    targetConfigDir,
    codeRoot,
    worktreesRoot,
    sessionsRoot,
    cmuxScope,
    launcher
  });

  if (flags.json) {
    console.log(JSON.stringify(plan, null, 2));
  } else {
    printPreflight(plan);
  }

  if (!plan.preflight.ok) {
    const error = new Error("aiw init preflight failed; install missing blocking dependencies before retrying");
    error.exitCode = 10;
    throw error;
  }

  if (flags.dryRun) {
    if (!flags.json) {
      printPlan(plan);
      console.log("[dry-run] no files were written");
    }
    return;
  }

  applyInitPlan(plan);
  if (!flags.json) {
    printPlan(plan);
    console.log("[ok] aiw init completed");
    if (plan.cmux.path) {
      console.log(`[ok] cmux registration: ${plan.cmux.path}`);
    }
  }

  if (plan.cmux.path && flags.reload !== false) {
    reloadCmux(flags.json);
  }
}

function buildInitPlan({ config, flags, targetConfigDir, codeRoot, worktreesRoot, sessionsRoot, cmuxScope, launcher }) {
  const sourceConfigDir = path.join(projectRoot(), "config");
  const cmuxPath = resolveCmuxPath(cmuxScope, codeRoot);
  const preflight = collectPreflight(config);
  const backupStamp = timestamp();
  const aiwFiles = AIW_CONFIG_FILES.map((name) => {
    const source = path.join(sourceConfigDir, name);
    const target = path.join(targetConfigDir, name);
    return {
      name,
      source,
      target,
      exists: fs.existsSync(target),
      action: fs.existsSync(target) && !flags.force ? "keep" : fs.existsSync(target) ? "overwrite" : "create",
      backup: fs.existsSync(target) && flags.force ? `${target}.${backupStamp}.bak` : ""
    };
  });

  return {
    preflight,
    options: {
      dryRun: Boolean(flags.dryRun),
      force: Boolean(flags.force),
      codeRoot,
      worktreesRoot,
      sessionsRoot,
      configDir: targetConfigDir,
      cmuxScope,
      launcher
    },
    directories: [
      { path: targetConfigDir, action: fs.existsSync(targetConfigDir) ? "keep" : "create" },
      { path: codeRoot, action: fs.existsSync(codeRoot) ? "keep" : "create" },
      { path: worktreesRoot, action: fs.existsSync(worktreesRoot) ? "keep" : "create" },
      { path: sessionsRoot, action: fs.existsSync(sessionsRoot) ? "keep" : "create" }
    ],
    aiwFiles,
    cmux: cmuxPath
      ? planCmuxConfig(cmuxPath, launcher, backupStamp)
      : { path: "", action: "skip", backup: "", plusButton: "skip" }
  };
}

function collectPreflight(config) {
  const platformOk = process.platform === "darwin" || process.platform === "linux";
  const env = [
    envCheck("HOME", process.env.HOME, "block"),
    envCheck("PATH", process.env.PATH, "block"),
    envCheck("SHELL", process.env.SHELL, "warn"),
    envCheck("AIW_CONFIG_DIR", process.env.AIW_CONFIG_DIR, "info")
  ];

  const nodeChecks = ["node", "npx"].map((name) => ({
    name,
    ok: Boolean(commandPath(name)),
    path: commandPath(name),
    blocking: true
  }));
  const layoutAgent = resolveAgent(config, config.defaults.agent);
  const commitAgent = resolveAgent(config, config.commit.agent || config.defaults.agent);
  const blockingAgents = uniqueAgents([layoutAgent, commitAgent]);
  const dependencyGate = gate("init", config, layoutAgent);
  const agentChecks = blockingAgents.map((agent) => ({
    name: agent.name,
    command: agent.cmd,
    ok: Boolean(commandPath(agent.cmd)),
    path: commandPath(agent.cmd)
  }));
  const missing = [
    ...(platformOk ? [] : [`unsupported platform: ${process.platform}`]),
    ...env.filter((item) => item.blocking && !item.ok).map((item) => item.name),
    ...nodeChecks.filter((item) => item.blocking && !item.ok).map((item) => item.name),
    ...dependencyGate.missing.filter((item) => !agentChecks.some((agent) => agent.command === item)),
    ...agentChecks.filter((item) => !item.ok).map((item) => `agent:${item.name}`)
  ];
  const uniqueMissing = [...new Set(missing)];

  return {
    ok: uniqueMissing.length === 0,
    platform: {
      name: process.platform,
      ok: platformOk,
      supported: ["darwin", "linux"]
    },
    env,
    node: nodeChecks,
    agents: agentChecks,
    gate: dependencyGate,
    optional: collectOptionalDependencies(config, new Set(blockingAgents.map((agent) => agent.name))),
    missing: uniqueMissing
  };
}

function collectOptionalDependencies(config, blockingAgentNames) {
  const agentCommands = Object.entries(config.agents)
    .filter(([name]) => !blockingAgentNames.has(name))
    .map(([name, agent]) => ({
      name: `agent:${name}`,
      command: String(agent.cmd || ""),
      ok: typeof agent.cmd === "string" && Boolean(commandPath(agent.cmd)),
      path: typeof agent.cmd === "string" ? commandPath(agent.cmd) : ""
    }));
  return [
    { name: "fd", command: "fd", ok: Boolean(commandPath("fd")), path: commandPath("fd") },
    { name: "eza", command: "eza", ok: Boolean(commandPath("eza")), path: commandPath("eza") },
    ...agentCommands
  ];
}

function envCheck(name, value, level) {
  return {
    name,
    ok: typeof value === "string" && value.length > 0,
    value: value || "",
    blocking: level === "block",
    level
  };
}

function resolveConfigDir(explicitConfigDir) {
  return path.resolve(expandHome(explicitConfigDir || process.env.AIW_CONFIG_DIR || path.join(os.homedir(), ".config", "aiw")));
}

async function selectCmuxScope(flags, codeRoot) {
  if (flags.cmuxScope) {
    return normalizeCmuxScope(flags.cmuxScope);
  }
  if (flags.yes || !process.stdin.isTTY) {
    return "home";
  }
  const homeLabel = `home - ${path.join(os.homedir(), ".config", "cmux", "cmux.json")} (default)`;
  const codeLabel = `code - ${path.join(codeRoot, ".cmux", "cmux.json")}`;
  const noneLabel = "none - skip cmux registration";
  const selected = await pickFromList("Register cmux config", [homeLabel, codeLabel, noneLabel], {
    defaultItem: homeLabel,
    force: true
  });
  if (selected === codeLabel) {
    return "code";
  }
  if (selected === noneLabel) {
    return "none";
  }
  return "home";
}

function normalizeCmuxScope(value) {
  const scope = String(value || "").trim().toLowerCase();
  if (scope === "home" || scope === "global") {
    return "home";
  }
  if (scope === "code" || scope === "code-root" || scope === "project") {
    return "code";
  }
  if (scope === "none" || scope === "skip") {
    return "none";
  }
  const error = new Error("--cmux-scope must be one of: home, code, none");
  error.exitCode = 2;
  throw error;
}

function resolveCmuxPath(scope, codeRoot) {
  if (scope === "home") {
    return path.join(os.homedir(), ".config", "cmux", "cmux.json");
  }
  if (scope === "code") {
    return path.join(codeRoot, ".cmux", "cmux.json");
  }
  return "";
}

function planCmuxConfig(cmuxPath, launcher, backupStamp) {
  const exists = fs.existsSync(cmuxPath);
  if (!exists) {
    return {
      path: cmuxPath,
      action: "create",
      backup: "",
      plusButton: "set"
    };
  }
  const current = readJson(cmuxPath);
  const existingAction = current.ui?.newWorkspace?.action;
  return {
    path: cmuxPath,
    action: "merge",
    backup: `${cmuxPath}.${backupStamp}.bak`,
    plusButton: existingAction && !AIW_ACTION_IDS.includes(existingAction) ? "preserve-existing" : "set",
    existingAction: existingAction || "",
    launcher
  };
}

function applyInitPlan(plan) {
  for (const directory of plan.directories) {
    fs.mkdirSync(directory.path, { recursive: true });
  }
  for (const file of plan.aiwFiles) {
    if (file.action === "keep") {
      continue;
    }
    if (file.backup) {
      fs.copyFileSync(file.target, file.backup);
    }
    const source = fs.readFileSync(file.source, "utf8");
    const next = file.name === "aiw.toml"
      ? renderAiwToml(source, plan.options.codeRoot, plan.options.worktreesRoot, plan.options.sessionsRoot, plan.options.configDir)
      : source;
    fs.writeFileSync(file.target, next);
  }
  if (plan.cmux.path) {
    writeCmuxConfig(plan.cmux.path, plan.options.launcher, plan.cmux);
  }
}

function renderAiwToml(source, codeRoot, worktreesRoot, sessionsRoot, configDir) {
  return source
    .replace(/^code_root\s*=\s*".*"$/m, `code_root = "${escapeToml(codeRoot)}"`)
    .replace(/^worktrees\s*=\s*".*"$/m, `worktrees = "${escapeToml(worktreesRoot)}"`)
    .replace(/^sessions\s*=\s*".*"$/m, `sessions = "${escapeToml(sessionsRoot)}"`)
    .replace(/^core_config\s*=\s*".*"$/m, `core_config = "${escapeToml(configDir)}"`);
}

function writeCmuxConfig(cmuxPath, launcher, cmuxPlan) {
  const existing = fs.existsSync(cmuxPath) ? readJson(cmuxPath) : {};
  if (cmuxPlan.backup) {
    fs.copyFileSync(cmuxPath, cmuxPlan.backup);
  }
  const next = mergeCmuxConfig(existing, launcher, cmuxPlan);
  fs.mkdirSync(path.dirname(cmuxPath), { recursive: true });
  fs.writeFileSync(cmuxPath, `${JSON.stringify(next, null, 2)}\n`);
}

function mergeCmuxConfig(existing, launcher, cmuxPlan) {
  const next = isPlainObject(existing) ? { ...existing } : {};
  const actions = isPlainObject(next.actions) ? { ...next.actions } : {};
  actions["aiw-new-worktree"] = cmuxAction({
    title: "AIW New Worktree",
    subtitle: "Create a Worktrunk worktree from the current workspace",
    command: `${launcher} cmux-new`,
    icon: "folder.badge.plus"
  });
  actions["aiw-pick-directory"] = cmuxAction({
    title: "AIW Pick Directory",
    subtitle: "Choose a repository before running aiw cmux-new",
    command: `${launcher} cmux-new --pick-repo`,
    icon: "folder.badge.plus"
  });
  actions["aiw-local-workspace"] = cmuxAction({
    title: "AIW Local Workspace",
    subtitle: "Open the current checkout without creating a worktree",
    command: `${launcher} cmux-new --local`,
    icon: "terminal"
  });
  actions["aiw-scratch-session"] = cmuxAction({
    title: "AIW Scratch Session",
    subtitle: "Open a non-project AIW session",
    command: `${launcher} cmux scratch`,
    icon: "square.and.pencil"
  });
  next.actions = actions;

  const ui = isPlainObject(next.ui) ? { ...next.ui } : {};
  const newWorkspace = isPlainObject(ui.newWorkspace) ? { ...ui.newWorkspace } : {};
  if (cmuxPlan.plusButton !== "preserve-existing") {
    newWorkspace.action = "aiw-new-worktree";
  }
  newWorkspace.contextMenu = mergeContextMenu(newWorkspace.contextMenu);
  ui.newWorkspace = newWorkspace;
  next.ui = ui;
  return next;
}

function cmuxAction({ title, subtitle, command, icon }) {
  return {
    type: "command",
    title,
    subtitle,
    command,
    target: "newTabInCurrentPane",
    icon: {
      type: "symbol",
      name: icon
    }
  };
}

function mergeContextMenu(currentMenu) {
  const existingItems = Array.isArray(currentMenu) ? currentMenu : [];
  const nonAiwItems = existingItems.filter((item) => {
    if (!isPlainObject(item) || typeof item.action !== "string") {
      return true;
    }
    return !AIW_ACTION_IDS.includes(item.action);
  });
  if (nonAiwItems.length === 0) {
    return DEFAULT_CONTEXT_MENU;
  }
  return [
    ...defaultAiwContextMenuItems(),
    { type: "separator" },
    ...trimLeadingSeparators(nonAiwItems)
  ];
}

function defaultAiwContextMenuItems() {
  return DEFAULT_CONTEXT_MENU.filter((item) => isPlainObject(item) && AIW_ACTION_IDS.includes(item.action));
}

function trimLeadingSeparators(items) {
  let start = 0;
  while (start < items.length && isPlainObject(items[start]) && items[start].type === "separator") {
    start += 1;
  }
  return items.slice(start);
}

function readJson(filePath) {
  const source = fs.readFileSync(filePath, "utf8");
  try {
    return JSON.parse(source);
  } catch (error) {
    try {
      return JSON.parse(removeTrailingCommas(stripJsonComments(source)));
    } catch {
      const wrapped = new Error(`invalid JSON in ${filePath}: ${error.message}`);
      wrapped.exitCode = 2;
      throw wrapped;
    }
  }
}

function stripJsonComments(source) {
  let result = "";
  let inString = false;
  let escaped = false;
  let inLineComment = false;
  let inBlockComment = false;
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index];
    const next = source[index + 1];
    if (inLineComment) {
      if (char === "\n" || char === "\r") {
        inLineComment = false;
        result += char;
      }
      continue;
    }
    if (inBlockComment) {
      if (char === "*" && next === "/") {
        inBlockComment = false;
        index += 1;
      }
      continue;
    }
    if (escaped) {
      escaped = false;
      result += char;
      continue;
    }
    if (char === "\\") {
      escaped = true;
      result += char;
      continue;
    }
    if (char === '"') {
      inString = !inString;
      result += char;
      continue;
    }
    if (!inString && char === "/" && next === "/") {
      inLineComment = true;
      index += 1;
      continue;
    }
    if (!inString && char === "/" && next === "*") {
      inBlockComment = true;
      index += 1;
      continue;
    }
    result += char;
  }
  return result;
}

function removeTrailingCommas(source) {
  let result = "";
  let inString = false;
  let escaped = false;
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index];
    if (escaped) {
      escaped = false;
      result += char;
      continue;
    }
    if (char === "\\") {
      escaped = true;
      result += char;
      continue;
    }
    if (char === '"') {
      inString = !inString;
      result += char;
      continue;
    }
    if (!inString && char === ",") {
      const nextIndex = nextNonWhitespaceIndex(source, index + 1);
      if (source[nextIndex] === "}" || source[nextIndex] === "]") {
        continue;
      }
    }
    result += char;
  }
  return result;
}

function nextNonWhitespaceIndex(source, start) {
  for (let index = start; index < source.length; index += 1) {
    if (!/\s/.test(source[index])) {
      return index;
    }
  }
  return source.length;
}

function reloadCmux(json) {
  const result = tryCapture("cmux", ["reload-config"]);
  if (json) {
    console.log(JSON.stringify({ cmuxReload: result }, null, 2));
    return;
  }
  if (result.ok) {
    console.log("[ok] cmux config reloaded");
    return;
  }
  console.warn(`[warn] cmux reload-config failed: ${result.stderr || result.stdout || `exit ${result.status}`}`);
}

function printPreflight(plan) {
  const { preflight } = plan;
  console.log("AIW init preflight");
  console.log(`${preflight.platform.ok ? "[ok]" : "[missing]"} platform ${preflight.platform.name} (supported: ${preflight.platform.supported.join(", ")})`);
  for (const item of preflight.env) {
    if (item.level === "info" && !item.value) {
      continue;
    }
    const status = item.ok ? "[ok]" : item.blocking ? "[missing]" : "[warn]";
    const suffix = item.value ? `=${formatEnvValue(item)}` : "";
    console.log(`${status} env ${item.name}${suffix}`);
  }
  for (const item of preflight.node) {
    console.log(`${item.ok ? "[ok]" : "[missing]"} ${item.name}${item.path ? ` ${item.path}` : ""}`);
  }
  for (const item of preflight.agents) {
    console.log(`${item.ok ? "[ok]" : "[missing]"} agent:${item.name} (${item.command})${item.path ? ` ${item.path}` : ""}`);
  }
  for (const item of preflight.gate.satisfied) {
    if (preflight.agents.some((agent) => agent.command === item)) {
      continue;
    }
    console.log(`[ok] ${item}`);
  }
  for (const item of preflight.gate.missing) {
    if (preflight.agents.some((agent) => agent.command === item)) {
      continue;
    }
    console.log(`[missing] ${item}`);
  }
  const optionalMissing = preflight.optional.filter((item) => !item.ok);
  for (const item of optionalMissing) {
    console.log(`[optional missing] ${item.name}${item.command ? ` (${item.command})` : ""}`);
  }
  if (preflight.missing.length > 0) {
    console.log("");
    console.log("Install missing blocking dependencies:");
    for (const item of preflight.missing) {
      console.log(`- ${item}: ${hintFor(item)}`);
    }
    return;
  }
  console.log("[ok] blocking dependency gate passed");
}

function printPlan(plan) {
  console.log("");
  console.log("AIW init plan");
  for (const directory of plan.directories) {
    console.log(`[${directory.action}] dir ${directory.path}`);
  }
  for (const file of plan.aiwFiles) {
    const backup = file.backup ? ` backup=${file.backup}` : "";
    console.log(`[${file.action}] ${file.target}${backup}`);
  }
  if (plan.cmux.path) {
    const backup = plan.cmux.backup ? ` backup=${plan.cmux.backup}` : "";
    const plus = plan.cmux.plusButton === "preserve-existing"
      ? ` preserve plus-button action=${plan.cmux.existingAction}`
      : " set plus-button action=aiw-new-worktree";
    console.log(`[${plan.cmux.action}] ${plan.cmux.path}${backup}${plus}`);
  } else {
    console.log("[skip] cmux registration");
  }
  console.log(`[skip] skills initialization`);
}

function hintFor(item) {
  if (item.includes(" or ")) {
    return item.split(" or ").map((part) => hintFor(part)).join(" OR ");
  }
  if (item.startsWith("unsupported platform")) {
    return "aiw init currently supports macOS and Linux only.";
  }
  if (item.startsWith("agent:")) {
    return "Install the configured agent CLI or change ~/.config/aiw/agents.toml.";
  }
  return INSTALL_HINTS[item] || "Install it and ensure it is available on PATH.";
}

function uniqueAgents(agents) {
  const seen = new Set();
  const result = [];
  for (const agent of agents) {
    const key = `${agent.name}:${agent.cmd}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    result.push(agent);
  }
  return result;
}

function formatEnvValue(item) {
  if (item.name === "PATH") {
    return `set (${item.value.split(path.delimiter).filter(Boolean).length} entries)`;
  }
  if (item.value.length > 120) {
    return `${item.value.slice(0, 117)}...`;
  }
  return item.value;
}

function parseInitFlags(argv) {
  const flags = {
    reload: true
  };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case "--help":
      case "-h":
        flags.help = true;
        break;
      case "--cmux-scope":
      case "--cmux":
        flags.cmuxScope = argv[++index];
        break;
      case "--config-dir":
        flags.configDir = argv[++index];
        break;
      case "--code-root":
        flags.codeRoot = argv[++index];
        break;
      case "--worktrees-root":
        flags.worktreesRoot = argv[++index];
        break;
      case "--sessions-root":
        flags.sessionsRoot = argv[++index];
        break;
      case "--launcher":
      case "--command-prefix":
        flags.launcher = argv[++index];
        break;
      case "--force":
      case "-f":
        flags.force = true;
        break;
      case "--yes":
      case "-y":
        flags.yes = true;
        break;
      case "--dry-run":
        flags.dryRun = true;
        break;
      case "--json":
        flags.json = true;
        break;
      case "--no-reload":
        flags.reload = false;
        break;
      default: {
        const error = new Error(`unknown init option: ${arg}`);
        error.exitCode = 2;
        throw error;
      }
    }
  }
  return flags;
}

function printInitHelp() {
  console.log(`Usage: aiw init [options]

Initialize AIW on macOS/Linux through the npx-friendly entrypoint.

Options:
  --cmux-scope <home|code|none>   Register cmux in ~/.config/cmux, <code-root>/.cmux, or skip
  --config-dir <path>             AIW config directory; defaults to AIW_CONFIG_DIR or ~/.config/aiw
  --code-root <path>              Code root written to aiw.toml; defaults to ~/Code
  --worktrees-root <path>         Worktree root written to aiw.toml; defaults to ~/worktrees
  --sessions-root <path>          Scratch session root written to aiw.toml; defaults to ~/Documents/aiw
  --launcher <command>            Command prefix stored in cmux actions; defaults to "npx aiw"
  --force                         Overwrite existing AIW config files after creating backups
  --yes                           Use defaults without prompts
  --dry-run                       Print the plan without writing files
  --no-reload                     Do not run cmux reload-config after writing
  --json                          Print structured preflight and plan data`);
}

function escapeToml(value) {
  return String(value).replace(/\\/g, "\\\\").replace(/"/g, "\\\"");
}

function timestamp() {
  return new Date().toISOString().replace(/[-:]/g, "").replace(/\..+$/, "");
}

function isPlainObject(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
