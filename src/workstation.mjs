import fs from "node:fs";
import path from "node:path";
import { buildCmuxLayout, buildProjectLayoutModel, buildScratchLayoutModel, scratchWorkspaceName, workspaceName } from "./layout.mjs";
import { commandExists, quoteShell, runInherit, tryCapture } from "./run.mjs";

export function resolveWorkstation(config, overrides = {}) {
  if (overrides.implementation) {
    return validateWorkstation({ implementation: overrides.implementation });
  }
  return runtimeWorkstation();
}

export function validateWorkstation(workstation) {
  const implementation = String(workstation.implementation || "").trim().toLowerCase();
  if (implementation !== "tmux" && implementation !== "cmux") {
    const error = new Error(
      `invalid workstation implementation: ${implementation || "(missing)"}; valid implementations: tmux, cmux`
    );
    error.exitCode = 2;
    throw error;
  }
  return { implementation };
}

export function recommendedWorkstation() {
  return runtimeWorkstation();
}

export function workstationRequirements(profile, config, agent) {
  const agentCmd = agent?.cmd;
  const gitDeps = [config.defaults.git || "lazygit", ...lazygitOverlayDeps(config)];
  const recommended = ["cmux"];
  switch (profile) {
    case "init":
      return req(["sh", "git", "wt", "tmux", "yazi", "nvim", ...gitDeps, "rg", "fzf", "bat", agentCmd].filter(Boolean), recommended);
    case "layout":
      return req(["git", "tmux", "yazi", "nvim", ...gitDeps, agentCmd].filter(Boolean), recommended);
    case "scratch":
    case "session":
      return req(["tmux", "yazi", "nvim", agentCmd].filter(Boolean), recommended);
    case "scratch-resume":
    case "session-resume":
      return req(["tmux", "yazi", "nvim", "fzf", agentCmd].filter(Boolean), recommended);
    case "cmux-new":
    case "new":
      return req(["git", "wt", "tmux", "yazi", "nvim", ...gitDeps, agentCmd].filter(Boolean), recommended);
    default:
      return req([]);
  }
}

export function buildOpenPlan(config, options) {
  const workstation = resolveWorkstation(config);
  const model = options.kind === "scratch"
    ? buildScratchLayoutModel(config, options.agentName)
    : buildProjectLayoutModel(config, options.agentName);
  const cwd = options.cwd;
  const name = options.kind === "scratch"
    ? scratchWorkspaceName(cwd, options.agentName)
    : workspaceName(cwd, options.agentName);
  if (workstation.implementation === "cmux") {
    return buildCmuxOpenPlan({ workstation, model, name, cwd });
  }
  return buildTmuxOpenPlan({ workstation, model, name, cwd });
}

export async function openProjectWorkspace(config, options) {
  await applyOpenPlan(buildOpenPlan(config, { ...options, kind: "project" }), options);
}

export async function openScratchWorkspace(config, options) {
  await applyOpenPlan(buildOpenPlan(config, { ...options, kind: "scratch" }), options);
}

export function printOpenPlan(plan) {
  for (const command of plan.commands) {
    console.log(command.display);
  }
}

export function collectOpenWorkspacePaths(config) {
  const workstation = resolveWorkstation(config);
  if (workstation.implementation === "cmux") {
    return collectCmuxWorkspacePaths();
  }
  return collectTmuxWorkspacePaths(workstation.implementation);
}

export function workspaceRefForPath(config, workspacePath) {
  const workstation = resolveWorkstation(config);
  const targetPath = normalizePath(workspacePath);
  if (workstation.implementation === "cmux") {
    return cmuxWorkspaceRefForPath(targetPath);
  }
  return tmuxSessionRefForPath(targetPath);
}

export function closeWorkspaceRef(config, workspaceRef) {
  if (!workspaceRef) {
    return;
  }
  const workstation = resolveWorkstation(config);
  if (workstation.implementation === "cmux") {
    tryCapture("cmux", ["close-workspace", "--workspace", workspaceRef]);
    return;
  }
  tryCapture("tmux", ["kill-session", "-t", workspaceRef]);
}

export function workstationLabel(config) {
  const workstation = resolveWorkstation(config);
  return workstation.implementation;
}

function lazygitOverlayDeps(config) {
  return config.git.lazygit_config ? ["delta"] : [];
}

function runtimeWorkstation() {
  if (process.env.AIW_WORKSTATION === "cmux" && commandExists("cmux")) {
    return { implementation: "cmux" };
  }
  if (process.env.AIW_WORKSTATION === "tmux") {
    return { implementation: "tmux" };
  }
  if (isCmuxRuntime() && commandExists("cmux")) {
    return { implementation: "cmux" };
  }
  return { implementation: "tmux" };
}

function isCmuxRuntime() {
  return Boolean(
    process.env.CMUX_WORKSPACE_ID ||
    process.env.CMUX_SURFACE_ID ||
    process.env.CMUX_PANEL_ID ||
    process.env.CMUX_SHELL_INTEGRATION ||
    process.env.CMUX_SOCKET_PATH ||
    process.env.CMUX_BUNDLE_ID
  );
}

function req(commands, recommended = []) {
  return { commands, recommended };
}

function buildCmuxOpenPlan({ workstation, model, name, cwd }) {
  const layoutJson = JSON.stringify(buildCmuxLayout(model));
  const args = [
    "new-workspace",
    "--name",
    name,
    "--cwd",
    cwd,
    "--focus",
    "true",
    "--layout",
    layoutJson
  ];
  return {
    workstation,
    kind: model.type,
    name,
    cwd,
    model,
    commands: [commandPlan("cmux", args)]
  };
}

function buildTmuxOpenPlan({ workstation, model, name, cwd }) {
  const session = tmuxSessionName(name);
  const createCommands = model.type === "scratch"
    ? tmuxScratchCommands(session, cwd, model)
    : tmuxProjectCommands(session, cwd, model);
  const attach = tmuxAttachCommand(session);
  return {
    workstation,
    kind: model.type,
    name,
    cwd,
    model,
    session,
    commands: [
      ...createCommands,
      attach
    ]
  };
}

function tmuxProjectCommands(session, cwd, model) {
  return [
    commandPlan("tmux", ["new-session", "-d", "-s", session, "-c", cwd, model.panes[0].command]),
    commandPlan("tmux", ["split-window", "-v", "-p", "44", "-t", `${session}:0.0`, "-c", cwd, model.panes[2].command]),
    commandPlan("tmux", ["select-pane", "-t", `${session}:0.0`]),
    commandPlan("tmux", ["split-window", "-h", "-p", "66", "-t", `${session}:0.0`, "-c", cwd, model.panes[1].command]),
    commandPlan("tmux", ["select-pane", "-t", `${session}:0.1`])
  ];
}

function tmuxScratchCommands(session, cwd, model) {
  return [
    commandPlan("tmux", ["new-session", "-d", "-s", session, "-c", cwd, model.panes[0].command]),
    commandPlan("tmux", ["split-window", "-h", "-p", "66", "-t", `${session}:0.0`, "-c", cwd, model.panes[1].command]),
    commandPlan("tmux", ["select-pane", "-t", `${session}:0.1`])
  ];
}

function tmuxAttachCommand(session) {
  if (process.env.TMUX) {
    return commandPlan("tmux", ["switch-client", "-t", session]);
  }
  return commandPlan("tmux", ["attach-session", "-t", session]);
}

async function applyOpenPlan(plan, options = {}) {
  if (options.dryRun) {
    printOpenPlan(plan);
    return;
  }
  if (plan.workstation.implementation !== "cmux" && tmuxSessionExists(plan.session)) {
    const attach = plan.commands.at(-1);
    await runInherit(attach.command, attach.args);
    return;
  }
  for (const command of plan.commands) {
    await runInherit(command.command, command.args, { cwd: plan.cwd });
  }
}

function commandPlan(command, args) {
  return {
    command,
    args,
    display: `${command} ${args.map(quoteShell).join(" ")}`
  };
}

function tmuxSessionExists(session) {
  const result = tryCapture("tmux", ["has-session", "-t", session]);
  return result.ok;
}

function tmuxSessionName(name) {
  const slug = String(name || "aiw")
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^[.-]+|[.-]+$/g, "")
    .slice(0, 80);
  return slug.startsWith("ai-") ? `aiw-${slug.slice(3)}` : `aiw-${slug || "workspace"}`;
}

function collectCmuxWorkspacePaths() {
  const paths = new Map();
  const result = tryCapture("cmux", ["list-workspaces", "--json"]);
  if (!result.ok || !result.stdout) {
    return paths;
  }
  try {
    const parsed = JSON.parse(result.stdout);
    const workspaces = Array.isArray(parsed.workspaces) ? parsed.workspaces : [];
    for (const workspace of workspaces) {
      if (workspace.current_directory) {
        paths.set(normalizePath(workspace.current_directory), {
          implementation: "cmux",
          ref: String(workspace.ref || "")
        });
      }
    }
  } catch {
    return paths;
  }
  return paths;
}

function collectTmuxWorkspacePaths(implementation) {
  const paths = new Map();
  const result = tryCapture("tmux", ["list-panes", "-a", "-F", "#{session_name}\t#{pane_current_path}"]);
  if (!result.ok || !result.stdout) {
    return paths;
  }
  for (const line of result.stdout.split(/\r?\n/)) {
    const [session, panePath] = line.split("\t");
    if (session && panePath) {
      paths.set(normalizePath(panePath), {
        implementation,
        ref: session
      });
    }
  }
  return paths;
}

function cmuxWorkspaceRefForPath(workspacePath) {
  const found = collectCmuxWorkspacePaths().get(normalizePath(workspacePath));
  return found?.ref || "";
}

function tmuxSessionRefForPath(workspacePath) {
  const found = collectTmuxWorkspacePaths("tmux").get(normalizePath(workspacePath));
  return found?.ref || "";
}

function normalizePath(value) {
  if (!value) {
    return "";
  }
  const resolved = path.resolve(value);
  try {
    return fs.realpathSync.native(resolved);
  } catch {
    return resolved;
  }
}
