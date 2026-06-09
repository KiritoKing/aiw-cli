import fs from "node:fs";
import { randomUUID } from "node:crypto";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { expandHome, loadConfig, resolveAgent, aiwBinPath } from "./config.mjs";
import { runCommit, runCommitMessage } from "./commit.mjs";
import { assertGate, printDoctor } from "./deps.mjs";
import { assertGitRoot, gitRoot, isDirty, resolveRepo, selectBranch } from "./git.mjs";
import { runWorkspaceHook } from "./hooks.mjs";
import { commandInit } from "./init.mjs";
import { buildLayout, buildScratchLayout, scratchWorkspaceName, workspaceName } from "./layout.mjs";
import { commandExists, quoteShell, runInherit, sleep } from "./run.mjs";
import { commandWorkspace, recordWorkspaceTarget } from "./workspace.mjs";

export async function main(argv) {
  const command = normalizeCommand(argv[0] || "help");
  const rest = argv.slice(1);
  const config = loadConfig();

  switch (command) {
    case "help":
    case "-h":
    case "--help":
      printHelp();
      return;
    case "doctor":
      await commandDoctor(config, rest);
      return;
    case "init":
      await commandInit(config, rest);
      return;
    case "cmux-new":
    case "new":
      await commandCmuxNew(config, rest, command);
      return;
    case "cmux":
      if (normalizeCommand(rest[0] || "") === "new") {
        await commandCmuxNew(config, rest.slice(1), "cmux-new");
        return;
      }
      if (["scratch", "session"].includes(normalizeCommand(rest[0] || ""))) {
        await commandScratch(config, rest.slice(1));
        return;
      }
      break;
    case "layout":
      await commandLayout(config, rest);
      return;
    case "scratch":
    case "session":
      await commandScratch(config, rest);
      return;
    case "workspace":
    case "ws":
      await commandWorkspace(config, rest);
      return;
    case "open":
      await commandWorkspace(config, ["open", ...rest]);
      return;
    case "switch":
      await commandWorkspace(config, ["open", ...rest]);
      return;
    case "list":
    case "ls":
    case "als":
      await commandWorkspace(config, ["list", ...rest]);
      return;
    case "done":
      await commandWorkspace(config, ["done", ...rest]);
      return;
    case "remove":
      await commandWorkspace(config, ["remove", ...rest]);
      return;
    case "gc":
    case "clean":
      await commandWorkspace(config, ["gc", ...rest]);
      return;
    case "diff":
      await commandDiff(config, rest);
      return;
    case "commit":
      await commandCommit(config, rest);
      return;
    case "commit-message":
      await commandCommitMessage(config, rest);
      return;
    case "git":
      await commandGit(config, rest);
      return;
    case "files":
      assertGate("files", config);
      await runInherit(config.defaults.files || "yazi", [rest[0] || "."]);
      return;
    case "edit":
      await commandEdit(config, rest);
      return;
    case "grep":
      await commandGrep(config, rest);
      return;
    case "pick":
      await commandPick(config, rest);
      return;
    case "tree":
      await commandTree(config, rest);
      return;
  }

  const error = new Error(`unknown command: ${command}`);
  error.exitCode = 2;
  throw error;
}

async function commandDoctor(config, argv) {
  const flags = parseFlags(argv);
  const agent = flags.agent ? resolveAgent(config, flags.agent) : undefined;
  try {
    printDoctor(config, {
      json: flags.json,
      gate: flags.gate,
      agent
    });
  } catch (error) {
    if (error.exitCode === 10) {
      process.exitCode = 10;
      return;
    }
    throw error;
  }
}

async function commandCmuxNew(config, argv, commandName) {
  const flags = parseFlags(argv);
  const branchFromArgs = flags.positionals[0] && !isKnownAgent(config, flags.positionals[0])
    ? flags.positionals[0]
    : "";
  const agentFromArgs = flags.positionals.find((item) => isKnownAgent(config, item));
  if (flags.local && (flags.branch || branchFromArgs || flags.create || flags.base)) {
    const error = new Error("--local cannot be combined with --branch, a branch argument, --create, or --base");
    error.exitCode = 2;
    throw error;
  }
  const repo = await resolveRepo(process.cwd(), config.paths.code_root, flags.repo, {
    pickRepo: flags.pickRepo,
    interactive: !flags.repo
  });
  const branchSelection = flags.local
    ? { local: true }
    : await selectBranch(repo, flags.branch || branchFromArgs, {
        forceCreate: flags.create,
        baseBranch: flags.base
      });
  const agent = await selectAgent(config, flags.agent || agentFromArgs);

  if (branchSelection.local) {
    assertGate("layout", config, agent);
    const layoutArgs = ["layout", "--agent", agent.name];
    if (flags.dryRun) {
      console.log(`cd ${quoteShell(repo)} && ${quoteShell(aiwBinPath())} ${layoutArgs.map(quoteShell).join(" ")}`);
      return;
    }
    await runInherit(aiwBinPath(), layoutArgs, { cwd: repo });
    return;
  }

  assertGate("cmux-new", config, agent);

  if (config.behavior.warn_dirty_before_new !== false && isDirty(repo)) {
    console.warn(`[warn] ${repo} has uncommitted changes; Worktrunk will continue with the selected branch flow`);
  }

  const layoutCommand = `${quoteShell(aiwBinPath())} layout --agent ${quoteShell(agent.name)}`;
  const wtArgs = branchSelection.create
    ? ["switch", "--create", branchSelection.branch, "--base", branchSelection.baseBranch || "@", "-x", layoutCommand]
    : ["switch", branchSelection.branch, "-x", layoutCommand];
  if (flags.dryRun) {
    console.log(`cd ${quoteShell(repo)} && wt ${wtArgs.map(quoteShell).join(" ")}`);
    return;
  }
  await runInherit("wt", wtArgs, { cwd: repo });
  if (branchSelection.create && branchSelection.targetBranch) {
    recordWorkspaceTarget(repo, branchSelection.branch, branchSelection.targetBranch);
  }
}

async function commandLayout(config, argv) {
  const flags = parseFlags(argv);
  const agent = resolveAgent(config, flags.agent || flags.positionals[0]);
  const cwd = process.cwd();
  let repo = "";
  if (!flags.printJson) {
    assertGate("layout", config, agent);
  }
  if (config.behavior.require_git_repo !== false) {
    if (!flags.printJson) {
      repo = assertGitRoot(cwd);
    }
  } else {
    repo = gitRoot(cwd) || cwd;
  }
  const layout = buildLayout(config, agent.name);
  const layoutJson = JSON.stringify(layout);
  if (flags.printJson) {
    console.log(JSON.stringify(layout, null, 2));
    return;
  }
  await runWorkspaceHook(config, "pre_init", {
    repo: repo || cwd,
    cwd: repo || cwd,
    workspacePath: repo || cwd,
    branch: "",
    agent: agent.name,
    dryRun: flags.dryRun
  });
  if (flags.dryRun) {
    console.log(`cmux new-workspace --name ${quoteShell(workspaceName(cwd, agent.name))} --cwd ${quoteShell(cwd)} --focus true --layout ${quoteShell(layoutJson)}`);
    return;
  }
  await runInherit("cmux", [
    "new-workspace",
    "--name",
    workspaceName(cwd, agent.name),
    "--cwd",
    cwd,
    "--focus",
    "true",
    "--layout",
    layoutJson
  ]);
}

async function commandScratch(config, argv) {
  const subcommand = normalizeCommand(argv[0] || "");
  if (subcommand === "resume" || subcommand === "open") {
    await commandScratchResume(config, argv.slice(1));
    return;
  }
  if (subcommand === "list" || subcommand === "ls") {
    commandScratchList(config, argv.slice(1));
    return;
  }

  const flags = parseFlags(argv);
  const agentFromArgs = flags.positionals.find((item) => isKnownAgent(config, item));
  const idFromArgs = flags.positionals.find((item) => !isKnownAgent(config, item));
  const agent = await selectAgent(config, flags.agent || agentFromArgs);
  assertGate("scratch", config, agent);

  const root = path.resolve(expandHome(flags.root || config.paths.sessions));
  const now = new Date();
  const date = localDateStamp(now);
  const sessionId = normalizeSessionId(flags.id || idFromArgs || generatedSessionId(now));
  const sessionPath = path.join(root, date, sessionId);
  const firstMessage = normalizeFirstMessage(flags.message || flags.firstMessage || idFromArgs || "");

  if (flags.dryRun) {
    console.log(`mkdir -p ${quoteShell(sessionPath)}`);
    console.log(`write ${quoteShell(sessionMetadataPath(sessionPath))}`);
    printScratchOpenCommand(config, agent.name, sessionPath);
    return;
  }

  fs.mkdirSync(sessionPath, { recursive: true });
  writeSessionMetadata(sessionPath, {
    id: sessionId,
    agent: agent.name,
    createdAt: now.toISOString(),
    firstMessage,
    root,
    path: sessionPath
  });
  console.log(`[aiw scratch] ${sessionPath}`);
  await openScratchSession(config, agent.name, sessionPath);
}

async function commandScratchResume(config, argv) {
  const flags = parseFlags(argv);
  const agent = await selectAgent(config, flags.agent);
  assertGate(flags.id ? "scratch" : "scratch-resume", config, agent);
  const root = path.resolve(expandHome(flags.root || config.paths.sessions));
  const sessions = listScratchSessions(root);
  if (sessions.length === 0) {
    const error = new Error(`no scratch sessions found under ${root}`);
    error.exitCode = 4;
    throw error;
  }
  const selected = flags.id
    ? selectSessionByIdOrPath(sessions, flags.id)
    : pickScratchSession(sessions, flags.query || flags.positionals.join(" "));
  if (flags.dryRun) {
    printScratchOpenCommand(config, agent.name, selected.path);
    return;
  }
  await openScratchSession(config, agent.name, selected.path);
}

function commandScratchList(config, argv) {
  const flags = parseFlags(argv);
  const root = path.resolve(expandHome(flags.root || config.paths.sessions));
  const sessions = listScratchSessions(root);
  if (flags.json) {
    console.log(JSON.stringify(sessions, null, 2));
    return;
  }
  if (sessions.length === 0) {
    console.log(`No scratch sessions found under ${root}`);
    return;
  }
  for (const session of sessions) {
    console.log(sessionDisplayLine(session));
  }
}

async function openScratchSession(config, agentName, sessionPath) {
  const layout = buildScratchLayout(config, agentName);
  await runInherit("cmux", [
    "new-workspace",
    "--name",
    scratchWorkspaceName(sessionPath, agentName),
    "--cwd",
    sessionPath,
    "--focus",
    "true",
    "--layout",
    JSON.stringify(layout)
  ]);
}

function printScratchOpenCommand(config, agentName, sessionPath) {
  const layout = buildScratchLayout(config, agentName);
  const layoutJson = JSON.stringify(layout);
  console.log(`cmux new-workspace --name ${quoteShell(scratchWorkspaceName(sessionPath, agentName))} --cwd ${quoteShell(sessionPath)} --focus true --layout ${quoteShell(layoutJson)}`);
}

async function commandDiff(config, argv) {
  const flags = parseFlags(argv);
  assertGate("diff", config);
  const mode = flags.staged ? "--staged" : flags.all ? "--all" : "";
  if (flags.watch) {
    for (;;) {
      process.stdout.write("\x1Bc");
      console.log(`[aiw diff] ${new Date().toLocaleTimeString()} ${mode}`.trim());
      await runDiffOnce(mode);
      await sleep(2000);
    }
  }
  await runDiffOnce(mode);
}

async function commandGit(config, argv) {
  assertGate("git", config);
  const lazygit = config.defaults.git || "lazygit";
  const lazygitConfig = resolveConfigFile(config, config.git.lazygit_config);
  const args = lazygitConfig ? ["--use-config-file", lazygitConfig, ...argv] : argv;
  await runInherit(lazygit, args);
}

async function commandCommit(config, argv) {
  const flags = parseFlags(argv);
  const agent = resolveAgent(config, flags.agent || config.commit.agent || config.defaults.agent);
  assertGate("commit", config, agent);
  await runCommit(config, flags);
}

async function commandCommitMessage(config, argv) {
  const flags = parseFlags(argv);
  const agent = resolveAgent(config, flags.agent || config.commit.agent || config.defaults.agent);
  assertGate("commit", config, agent);
  runCommitMessage(config, flags);
}

function resolveConfigFile(config, value) {
  if (!value) {
    return "";
  }
  const expanded = expandHome(value);
  const resolved = path.isAbsolute(expanded) ? expanded : path.join(config.configDir, expanded);
  return fs.existsSync(resolved) ? resolved : "";
}

async function runDiffOnce(mode) {
  if (!mode && commandExists("cmux-git-diff")) {
    await runInherit("cmux-git-diff");
    return;
  }
  const args = mode === "--staged" ? ["diff", "--staged"] : mode === "--all" ? ["diff", "HEAD"] : ["diff"];
  if (commandExists("delta")) {
    await runInherit("sh", ["-lc", `git ${args.map(quoteShell).join(" ")} | delta`]);
    return;
  }
  await runInherit("git", args);
}

async function commandEdit(config, argv) {
  assertGate("edit", config);
  const target = argv[0];
  if (!target) {
    const error = new Error("Usage: aiw edit <file[:line]>");
    error.exitCode = 2;
    throw error;
  }
  const editor = config.defaults.editor || process.env.EDITOR || "nvim";
  const match = target.match(/^(.+):([0-9]+)$/);
  if (match) {
    await runInherit(editor, [`+${match[2]}`, match[1]]);
    return;
  }
  await runInherit(editor, [target]);
}

async function commandGrep(config, argv) {
  assertGate("grep", config);
  const query = argv.join(" ");
  if (!query) {
    const error = new Error("Usage: aiw grep <query>");
    error.exitCode = 2;
    throw error;
  }
  const editor = config.defaults.editor || process.env.EDITOR || "nvim";
  await runInherit("sh", [
    "-lc",
    `rg -n ${quoteShell(query)} | fzf --delimiter ':' --preview 'bat --style=numbers --color=always --highlight-line {2} {1}' --bind ${quoteShell(`enter:execute(${editor} +{2} {1})`)}`
  ]);
}

async function commandPick(config) {
  assertGate("pick", config);
  const editor = config.defaults.editor || process.env.EDITOR || "nvim";
  await runInherit("sh", [
    "-lc",
    `fd -t f | fzf --preview 'bat --style=numbers --color=always {}' --bind ${quoteShell(`enter:execute(${editor} {})`)}`
  ]);
}

async function commandTree(config, argv) {
  const depth = argv[0] || String(config.defaults.tree_depth || 3);
  if (commandExists("eza")) {
    await runInherit("eza", ["--tree", `--level=${depth}`, "--git-ignore"]);
    return;
  }
  await runInherit("find", [".", "-maxdepth", depth, "-type", "f"]);
}

function parseFlags(argv) {
  const flags = {
    positionals: []
  };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case "--agent":
        flags.agent = argv[++index];
        break;
      case "--branch":
        flags.branch = argv[++index];
        break;
      case "--base":
      case "--from":
        flags.base = argv[++index];
        break;
      case "--repo":
        flags.repo = argv[++index];
        break;
      case "--root":
        flags.root = argv[++index];
        break;
      case "--id":
      case "--session-id":
        flags.id = argv[++index];
        break;
      case "--message":
      case "--first-message":
        flags.message = argv[++index];
        flags.firstMessage = flags.message;
        break;
      case "--query":
        flags.query = argv[++index];
        break;
      case "--prompt":
        flags.prompt = argv[++index];
        break;
      case "--prompt-file":
        flags.promptFile = argv[++index];
        break;
      case "--retries":
        flags.retries = argv[++index];
        break;
      case "--pick-repo":
      case "--select-repo":
        flags.pickRepo = true;
        break;
      case "--create":
        flags.create = true;
        break;
      case "--local":
        flags.local = true;
        break;
      case "--gate":
        flags.gate = argv[++index];
        break;
      case "--json":
        flags.json = true;
        break;
      case "--print-json":
        flags.printJson = true;
        break;
      case "--print-prompt":
        flags.printPrompt = true;
        break;
      case "--dry-run":
        flags.dryRun = true;
        break;
      case "--watch":
        flags.watch = true;
        break;
      case "--staged":
        flags.staged = true;
        break;
      case "--all":
        flags.all = true;
        break;
      case "--force":
      case "-f":
        flags.force = true;
        break;
      default:
        flags.positionals.push(arg);
    }
  }
  return flags;
}

function isKnownAgent(config, value) {
  return Boolean(value && config.agents[value]);
}

async function selectAgent(config, requestedAgent) {
  if (requestedAgent) {
    return resolveAgent(config, requestedAgent);
  }
  if (!process.stdin.isTTY) {
    return resolveAgent(config, config.defaults.agent);
  }
  const { pickFromList } = await import("./prompt.mjs");
  const agents = Object.keys(config.agents);
  const selected = await pickFromList("Select agent", agents, {
    defaultItem: config.defaults.agent || "codex"
  });
  return resolveAgent(config, selected);
}

function localDateStamp(date) {
  return [
    date.getFullYear(),
    pad2(date.getMonth() + 1),
    pad2(date.getDate())
  ].join("-");
}

function generatedSessionId(date) {
  return `${pad2(date.getHours())}${pad2(date.getMinutes())}${pad2(date.getSeconds())}-${randomUUID().slice(0, 8)}`;
}

function normalizeSessionId(value) {
  const normalized = String(value || "")
    .trim()
    .replace(/[/\\]+/g, "-")
    .replace(/[^A-Za-z0-9._-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^[.-]+|[.-]+$/g, "");
  return normalized || generatedSessionId(new Date());
}

function sessionMetadataPath(sessionPath) {
  return path.join(sessionPath, ".aiw-session.json");
}

function writeSessionMetadata(sessionPath, metadata) {
  const payload = {
    schema: 1,
    type: "scratch",
    id: metadata.id,
    agent: metadata.agent,
    created_at: metadata.createdAt,
    first_message: metadata.firstMessage || "",
    root: metadata.root,
    path: metadata.path
  };
  fs.writeFileSync(sessionMetadataPath(sessionPath), `${JSON.stringify(payload, null, 2)}\n`);
}

function listScratchSessions(root) {
  if (!fs.existsSync(root)) {
    return [];
  }
  const sessions = [];
  for (const dateEntry of safeReadDir(root)) {
    const datePath = path.join(root, dateEntry.name);
    if (!dateEntry.isDirectory()) {
      continue;
    }
    for (const sessionEntry of safeReadDir(datePath)) {
      if (!sessionEntry.isDirectory()) {
        continue;
      }
      const sessionPath = path.join(datePath, sessionEntry.name);
      sessions.push(readScratchSession(root, dateEntry.name, sessionEntry.name, sessionPath));
    }
  }
  return sessions.sort((left, right) => right.createdAtMs - left.createdAtMs || right.path.localeCompare(left.path));
}

function readScratchSession(root, date, id, sessionPath) {
  const metadata = readSessionMetadata(sessionPath);
  const stat = safeStat(sessionPath);
  const createdAt = metadata.created_at || (stat ? stat.mtime.toISOString() : "");
  const createdAtMs = Date.parse(createdAt) || (stat ? stat.mtimeMs : 0);
  return {
    id: String(metadata.id || id),
    date,
    createdAt,
    createdAtMs,
    time: createdAt ? localDateTimeStamp(new Date(createdAtMs || createdAt)) : "",
    firstMessage: String(metadata.first_message || ""),
    agent: String(metadata.agent || ""),
    root,
    path: sessionPath
  };
}

function readSessionMetadata(sessionPath) {
  const metadataPath = sessionMetadataPath(sessionPath);
  if (!fs.existsSync(metadataPath)) {
    return {};
  }
  try {
    const parsed = JSON.parse(fs.readFileSync(metadataPath, "utf8"));
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
}

function pickScratchSession(sessions, query) {
  if (!process.stdin.isTTY) {
    const error = new Error("scratch resume requires an interactive terminal; pass --id to select non-interactively");
    error.exitCode = 4;
    throw error;
  }
  const lines = sessions.map(sessionTuiLine);
  const args = [
    "--prompt",
    "Scratch session> ",
    "--delimiter",
    "\t",
    "--with-nth",
    "1,2,3"
  ];
  if (query) {
    args.push("--query", query);
  }
  const result = spawnSync("fzf", args, {
    input: `${lines.join("\n")}\n`,
    encoding: "utf8",
    stdio: ["pipe", "pipe", "inherit"]
  });
  if (result.status !== 0 || !result.stdout.trim()) {
    const error = new Error("scratch resume cancelled");
    error.exitCode = 4;
    throw error;
  }
  const selectedPath = result.stdout.trim().split("\t").at(-1);
  const selected = sessions.find((session) => session.path === selectedPath);
  if (!selected) {
    const error = new Error("selected scratch session no longer exists");
    error.exitCode = 4;
    throw error;
  }
  return selected;
}

function selectSessionByIdOrPath(sessions, value) {
  const expanded = path.resolve(expandHome(value));
  if (path.isAbsolute(expandHome(value))) {
    const selectedByPath = sessions.find((session) => session.path === expanded);
    if (selectedByPath) {
      return selectedByPath;
    }
  }
  const selected = sessions.find((session) => session.id === value || path.basename(session.path) === value);
  if (selected) {
    return selected;
  }
  const error = new Error(`scratch session not found: ${value}`);
  error.exitCode = 4;
  throw error;
}

function sessionTuiLine(session) {
  return [
    session.time || session.date,
    session.id,
    normalizeFirstMessage(session.firstMessage) || "(no first message)",
    session.path
  ].join("\t");
}

function sessionDisplayLine(session) {
  return `${session.time || session.date}  ${session.id}  ${normalizeFirstMessage(session.firstMessage) || "(no first message)"}  ${session.path}`;
}

function normalizeFirstMessage(value) {
  return String(value || "").replace(/\s+/g, " ").trim();
}

function safeReadDir(dir) {
  try {
    return fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return [];
  }
}

function safeStat(targetPath) {
  try {
    return fs.statSync(targetPath);
  } catch {
    return null;
  }
}

function localDateTimeStamp(date) {
  return `${localDateStamp(date)} ${pad2(date.getHours())}:${pad2(date.getMinutes())}:${pad2(date.getSeconds())}`;
}

function pad2(value) {
  return String(value).padStart(2, "0");
}

function printHelp() {
  const executable = path.relative(process.cwd(), fileURLToPath(import.meta.url)).startsWith("..")
    ? "aiw"
    : "./bin/aiw";
  console.log(`Usage: ${executable} <command> [options]

Commands:
  init [--cmux-scope <home|code|none>] [--code-root <path>] [--worktrees-root <path>] [--sessions-root <path>] [--config-dir <path>] [--dry-run]
  doctor [--json] [--gate <p0|init|layout|scratch|scratch-resume|cmux-new|workspace|worktrunk|diff|commit>] [--agent <name>]
  cmux-new|new [--branch <branch>] [--base <branch>] [--agent <name>] [--repo <path>] [--pick-repo] [--create] [--local] [--dry-run]
  scratch|session|cmux scratch [id] [--agent <name>] [--root <path>] [--id <id>] [--message <text>] [--dry-run]
  scratch resume [--agent <name>] [--root <path>] [--id <id>] [--query <text>] [--dry-run]
  scratch list [--root <path>] [--json]
  layout [--agent <name>] [--print-json] [--dry-run]
  workspace|ws <list|open|done|remove|gc> [options]
  commit [--agent <name>] [--prompt <text>] [--prompt-file <path>] [--retries <n>] [--dry-run] [--print-prompt]
  commit-message [--agent <name>] [--prompt <text>]
  open | switch | list | ls | als | done | remove | gc | clean
  diff [--watch] [--staged] [--all]
  git | files [path] | edit <file[:line]> | grep <query> | pick | tree [depth]

Main workflow commands run dependency gates before creating worktrees or opening cmux layouts.`);
}

function normalizeCommand(command) {
  return String(command || "").toLowerCase();
}
