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
import { buildOpenPlan, closeTmuxSession, closeWorkspaceRef, collectManagedTmuxSessions, collectOpenWorkspacePaths, openProjectWorkspace, openScratchWorkspace, printOpenPlan, workspaceRefForPath } from "./workstation.mjs";
import { askInput } from "./prompt.mjs";
import { commandExists, quoteShell, runInherit, sleep } from "./run.mjs";
import { commandWorkspace, recordWorkspaceTarget } from "./workspace.mjs";

const DEFAULT_SCRATCH_TMUX_STALE_SECONDS = 7 * 24 * 60 * 60;

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
      await commandNew(config, rest);
      return;
    case "cmux":
      if (normalizeCommand(rest[0] || "") === "new") {
        await commandNew(config, rest.slice(1));
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

async function commandNew(config, argv) {
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

  assertGate("new", config, agent);

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
  if (flags.printJson) {
    console.log(JSON.stringify(buildOpenPlan(config, {
      kind: "project",
      agentName: agent.name,
      cwd
    }), null, 2));
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
    await openProjectWorkspace(config, {
      agentName: agent.name,
      cwd,
      dryRun: true
    });
    return;
  }
  await openProjectWorkspace(config, { agentName: agent.name, cwd });
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
  if (subcommand === "close") {
    commandScratchClose(config, argv.slice(1));
    return;
  }
  if (subcommand === "gc" || subcommand === "clean") {
    await commandScratchGc(config, argv.slice(1));
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
  const sessions = listScratchSessionsWithUi(config, root);
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

function commandScratchClose(config, argv) {
  const flags = parseFlags(argv);
  const root = path.resolve(expandHome(flags.root || config.paths.sessions));
  const sessions = listScratchSessionsWithUi(config, root);
  if (sessions.length === 0) {
    const error = new Error(`no scratch sessions found under ${root}`);
    error.exitCode = 4;
    throw error;
  }
  const target = flags.id || flags.positionals[0] || "";
  const selected = target
    ? selectSessionByIdOrPath(sessions, target)
    : pickScratchSession(sessions, flags.query || flags.positionals.join(" "));
  const workspaceRef = selected.uiRef || workspaceRefForPath(config, selected.path);
  if (!workspaceRef) {
    if (flags.json) {
      console.log(JSON.stringify({ ok: true, action: "noop", reason: "scratch session is not open", session: selected }, null, 2));
      return;
    }
    console.log(`Scratch session is not open: ${selected.id}`);
    return;
  }
  if (flags.dryRun) {
    if (flags.json) {
      console.log(JSON.stringify({ ok: true, action: "close", dryRun: true, uiRef: workspaceRef, session: selected }, null, 2));
      return;
    }
    console.log(`close ${selected.uiImplementation || "ui"} ${workspaceRef}`);
    return;
  }
  closeWorkspaceRef(config, workspaceRef);
  if (flags.json) {
    console.log(JSON.stringify({ ok: true, action: "close", uiRef: workspaceRef, session: selected }, null, 2));
    return;
  }
  console.log(`Closed scratch session UI: ${selected.id}`);
}

async function commandScratchGc(config, argv) {
  const flags = parseFlags(argv);
  if (!flags.tmux) {
    const error = new Error("aiw scratch gc currently requires --tmux");
    error.exitCode = 2;
    throw error;
  }
  if (!commandExists("tmux")) {
    const error = new Error("dependency gate 'scratch-gc' failed\n  [missing] tmux");
    error.exitCode = 10;
    throw error;
  }
  if (flags.dryRun && (flags.apply || flags.yes)) {
    const error = new Error("aiw scratch gc --tmux cannot combine --dry-run with --apply/--yes");
    error.exitCode = 2;
    throw error;
  }
  const root = path.resolve(expandHome(flags.root || config.paths.sessions));
  const staleSeconds = scratchTmuxStaleSecondsFromFlags(flags, config);
  const plan = buildScratchTmuxGcPlan(root, staleSeconds);
  if (flags.json && !flags.apply && !flags.yes) {
    console.log(JSON.stringify(plan, null, 2));
    return;
  }
  if (!flags.json) {
    console.log(formatScratchTmuxGcPreview(plan, { dryRun: flags.dryRun }));
  }
  if (flags.dryRun || plan.removable.length === 0) {
    if (flags.json && (flags.apply || flags.yes)) {
      console.log(JSON.stringify({ ...plan, closed: [], skipped: [] }, null, 2));
    }
    return;
  }
  if (!flags.apply && !flags.yes && !process.stdin.isTTY) {
    if (!flags.json) {
      console.log("Not interactive. Rerun with --apply or --yes to close stale scratch tmux sessions.");
    }
    return;
  }
  const shouldApply = flags.apply || flags.yes || await confirmScratchTmuxGcApply(plan);
  if (!shouldApply) {
    if (!flags.json) {
      console.log("Cancelled. No tmux sessions were closed.");
    }
    return;
  }
  const refreshedPlan = buildScratchTmuxGcPlan(root, staleSeconds);
  const result = applyScratchTmuxGcPlan(refreshedPlan);
  if (flags.json) {
    console.log(JSON.stringify({ ...refreshedPlan, ...result }, null, 2));
    return;
  }
  if (result.closed.length === 0) {
    console.log("No scratch tmux sessions were closed after refresh.");
    return;
  }
  console.log(`Closed ${result.closed.length} scratch tmux session(s): ${result.closed.join(", ")}`);
}

async function openScratchSession(config, agentName, sessionPath) {
  await openScratchWorkspace(config, { agentName, cwd: sessionPath });
}

function printScratchOpenCommand(config, agentName, sessionPath) {
  printOpenPlan(buildOpenPlan(config, {
    kind: "scratch",
    agentName,
    cwd: sessionPath
  }));
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
      case "--stale-seconds":
        flags.staleSeconds = Number(argv[++index]);
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
      case "--apply":
        flags.apply = true;
        break;
      case "--yes":
      case "-y":
        flags.yes = true;
        break;
      case "--tmux":
        flags.tmux = true;
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

function listScratchSessionsWithUi(config, root) {
  const uiPaths = collectOpenWorkspacePaths(config);
  return listScratchSessions(root).map((session) => {
    const ui = uiPaths.get(normalizeSessionPath(session.path));
    return {
      ...session,
      open: Boolean(ui),
      uiImplementation: ui?.implementation || "",
      uiRef: ui?.ref || ""
    };
  });
}

function buildScratchTmuxGcPlan(root, staleSeconds) {
  const normalizedRoot = normalizeSessionPath(root);
  const nowSeconds = Math.floor(Date.now() / 1000);
  const sessions = collectManagedTmuxSessions()
    .filter((session) => session.kind === "scratch")
    .filter((session) => session.cwd && pathWithinOrEqual(session.cwd, normalizedRoot))
    .map((session) => {
      const idleSeconds = session.activitySeconds > 0
        ? Math.max(0, nowSeconds - session.activitySeconds)
        : null;
      const stale = typeof idleSeconds === "number" && idleSeconds >= staleSeconds;
      const attached = session.attached > 0;
      return {
        ref: session.ref,
        name: session.name,
        path: session.cwd,
        activityAt: session.activityAt,
        idleSeconds,
        attached: session.attached,
        stale,
        removable: stale && !attached,
        reason: stale ? attached ? "attached" : "stale and unattached" : "not stale"
      };
    }).sort((left, right) => {
      const leftIdle = typeof left.idleSeconds === "number" ? left.idleSeconds : -1;
      const rightIdle = typeof right.idleSeconds === "number" ? right.idleSeconds : -1;
      return rightIdle - leftIdle || left.ref.localeCompare(right.ref);
    });
  return {
    kind: "scratch-tmux-gc",
    root: normalizedRoot,
    staleSeconds,
    total: sessions.length,
    removable: sessions.filter((session) => session.removable),
    kept: sessions.filter((session) => !session.removable)
  };
}

function applyScratchTmuxGcPlan(plan) {
  const closed = [];
  const skipped = [];
  for (const session of plan.removable) {
    const result = closeTmuxSession(session.ref);
    if (result.ok) {
      closed.push(session.ref);
    } else {
      skipped.push({
        ref: session.ref,
        reason: result.stderr || result.stdout || `tmux exited with ${result.status}`
      });
    }
  }
  return { closed, skipped };
}

function formatScratchTmuxGcPreview(plan, options = {}) {
  const lines = [
    `${options.dryRun ? "Scratch tmux GC dry-run" : "Scratch tmux GC plan"}: ${plan.removable.length} removable, ${plan.kept.length} kept. stale >= ${plan.staleSeconds}s`,
    `root: ${plan.root}`
  ];
  if (plan.removable.length === 0) {
    lines.push("Removable: none");
  } else {
    lines.push("Removable:");
    for (const session of plan.removable) {
      lines.push(`  ${session.ref}  idle=${formatDuration(session.idleSeconds)}  attached=${session.attached}  ${session.path}`);
    }
  }
  if (plan.kept.length > 0) {
    lines.push("Kept:");
    for (const session of plan.kept) {
      lines.push(`  ${session.ref}  ${session.reason}  idle=${formatDuration(session.idleSeconds)}  attached=${session.attached}  ${session.path}`);
    }
  }
  lines.push(options.dryRun
    ? "No tmux sessions were closed. Run without --dry-run to confirm, or use --apply/--yes."
    : "Only stale, unattached, AIW-managed scratch tmux sessions can be closed.");
  return lines.join("\n");
}

async function confirmScratchTmuxGcApply(plan) {
  const answer = await askInput(`Close ${plan.removable.length} stale scratch tmux session(s)? Type y to confirm`);
  return answer.toLowerCase() === "y";
}

function scratchTmuxStaleSecondsFromFlags(flags, config) {
  if (flags.staleSeconds === undefined) {
    const configured = Number(config.workspace?.stale_seconds);
    return Number.isFinite(configured) && configured >= 0
      ? Math.floor(configured)
      : DEFAULT_SCRATCH_TMUX_STALE_SECONDS;
  }
  if (!Number.isFinite(flags.staleSeconds) || flags.staleSeconds < 0) {
    const error = new Error("--stale-seconds must be a non-negative number");
    error.exitCode = 2;
    throw error;
  }
  return Math.floor(flags.staleSeconds);
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
    "1,2,3,4"
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
    session.open ? session.uiImplementation || "open" : "-",
    normalizeFirstMessage(session.firstMessage) || "(no first message)",
    session.path
  ].join("\t");
}

function sessionDisplayLine(session) {
  const ui = session.open ? session.uiImplementation || "open" : "-";
  return `${session.time || session.date}  ${session.id}  ${ui}  ${normalizeFirstMessage(session.firstMessage) || "(no first message)"}  ${session.path}`;
}

function normalizeSessionPath(value) {
  const resolved = path.resolve(expandHome(value));
  try {
    return fs.realpathSync.native(resolved);
  } catch {
    return resolved;
  }
}

function pathWithinOrEqual(target, root) {
  const normalizedTarget = normalizeSessionPath(target);
  const normalizedRoot = normalizeSessionPath(root);
  return normalizedTarget === normalizedRoot || normalizedTarget.startsWith(`${normalizedRoot}${path.sep}`);
}

function formatDuration(seconds) {
  if (typeof seconds !== "number") {
    return "unknown";
  }
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 48) {
    return `${hours}h`;
  }
  return `${Math.floor(hours / 24)}d`;
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
  doctor [--json] [--gate <p0|init|new|layout|scratch|scratch-resume|workspace|worktrunk|diff|commit>] [--agent <name>]
  new|cmux-new [--branch <branch>] [--base <branch>] [--agent <name>] [--repo <path>] [--pick-repo] [--create] [--local] [--dry-run]
  scratch|session|cmux scratch [id] [--agent <name>] [--root <path>] [--id <id>] [--message <text>] [--dry-run]
  scratch resume [--agent <name>] [--root <path>] [--id <id>] [--query <text>] [--dry-run]
  scratch list [--root <path>] [--json]
  scratch close [id|path] [--root <path>] [--dry-run] [--json]
  scratch gc --tmux [--root <path>] [--stale-seconds n] [--dry-run] [--apply|--yes] [--json]
  layout [--agent <name>] [--print-json] [--dry-run]
  workspace|ws <list|open|done|remove|gc> [options]
  commit [--agent <name>] [--prompt <text>] [--prompt-file <path>] [--retries <n>] [--dry-run] [--print-prompt]
  commit-message [--agent <name>] [--prompt <text>]
  open | switch | list | ls | als | done | remove | gc | clean
  diff [--watch] [--staged] [--all]
  git | files [path] | edit <file[:line]> | grep <query> | pick | tree [depth]

Main workflow commands run dependency gates before creating worktrees or opening workstation UI.`);
}

function normalizeCommand(command) {
  return String(command || "").toLowerCase();
}
