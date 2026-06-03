import fs from "node:fs";
import path from "node:path";
import { aiwBinPath, resolveAgent } from "./config.mjs";
import { assertGate } from "./deps.mjs";
import { assertGitRoot, isDirty } from "./git.mjs";
import { runWorkspaceHook } from "./hooks.mjs";
import { askInput, pickFromList } from "./prompt.mjs";
import { quoteShell, runInherit, tryCapture } from "./run.mjs";

const DEFAULT_STALE_SECONDS = 7 * 24 * 60 * 60;
const INTEGRATED_STATES = new Set(["integrated", "same_commit", "empty"]);

const ANSI = {
  reset: "\x1b[0m",
  bold: "\x1b[1m",
  dim: "\x1b[2m",
  red: "\x1b[31m",
  green: "\x1b[32m",
  yellow: "\x1b[33m",
  magenta: "\x1b[35m",
  cyan: "\x1b[36m",
  gray: "\x1b[90m"
};

export async function commandWorkspace(config, argv) {
  const subcommand = argv[0] || "list";
  const rest = argv.slice(1);
  switch (subcommand) {
    case "list":
    case "status":
      await workspaceList(config, rest);
      return;
    case "open":
    case "switch":
      await workspaceOpen(config, rest);
      return;
    case "done":
      await workspaceDone(config, rest);
      return;
    case "remove":
    case "rm":
      await workspaceRemove(config, rest);
      return;
    case "gc":
    case "clean":
      await workspaceGc(config, rest);
      return;
    case "states":
    case "state":
      printStateHelp();
      return;
    case "help":
    case "-h":
    case "--help":
      printWorkspaceHelp();
      return;
    default: {
      const error = new Error(`unknown workspace command: ${subcommand}`);
      error.exitCode = 2;
      throw error;
    }
  }
}

export function recordWorkspaceTarget(repo, branch, targetBranch) {
  if (!branch || !targetBranch || branch === targetBranch) {
    return;
  }
  const metadata = readWorkspaceMetadata(repo);
  metadata.workspaces[branch] = {
    ...(metadata.workspaces[branch] || {}),
    targetBranch,
    targetSource: "aiw",
    updatedAt: Math.floor(Date.now() / 1000),
    createdAt: metadata.workspaces[branch]?.createdAt || Math.floor(Date.now() / 1000)
  };
  writeWorkspaceMetadata(repo, metadata);
}

function workspaceList(config, argv) {
  assertGate("workspace", config);
  const flags = parseWorkspaceFlags(argv);
  const repo = assertGitRoot(process.cwd());
  const workspaces = collectWorkspaceRecords(repo);
  if (flags.json) {
    console.log(JSON.stringify(workspaces, null, 2));
    return;
  }
  console.log(formatWorkspaceTable(workspaces, {
    color: shouldUseColor(flags),
    staleSeconds: staleSecondsFromFlags(flags, config)
  }));
}

async function workspaceOpen(config, argv) {
  const flags = parseWorkspaceFlags(argv);
  const agent = resolveAgent(config, flags.agent || config.defaults.agent);
  assertGate("workspace", config);
  assertGate("layout", config, agent);

  const repo = assertGitRoot(process.cwd());
  const layoutCommand = `${quoteShell(aiwBinPath())} layout --agent ${quoteShell(agent.name)}`;
  const selected = flags.positionals[0] || "";

  if (!selected) {
    if (process.stdin.isTTY) {
      const pickedTarget = await selectWorkspaceOpenTarget(repo, flags);
      await openWorkspaceTarget(repo, pickedTarget, flags, agent, layoutCommand);
      return;
    }
    const wtArgs = pickerSwitchArgs(flags, layoutCommand);
    if (flags.dryRun) {
      console.log(`cd ${quoteShell(repo)} && wt ${wtArgs.map(quoteShell).join(" ")}`);
      return;
    }
    await runInherit("wt", wtArgs, { cwd: repo });
    return;
  }

  await openWorkspaceTarget(repo, selected, flags, agent, layoutCommand);
}

async function openWorkspaceTarget(repo, selected, flags, agent, layoutCommand) {
  const workspaces = collectWorkspaceRecords(repo);
  const record = findWorkspace(workspaces, selected);

  if (record?.branch) {
    const wtArgs = ["switch", record.branch, "-x", layoutCommand];
    if (flags.dryRun) {
      console.log(`cd ${quoteShell(repo)} && wt ${wtArgs.map(quoteShell).join(" ")}`);
      return;
    }
    await runInherit("wt", wtArgs, { cwd: repo });
    return;
  }

  const directPath = record?.path || selected;
  const directRoot = directPath ? gitRootIfExists(directPath) : "";
  if (directRoot) {
    const layoutArgs = ["layout", "--agent", agent.name];
    if (flags.dryRun) {
      console.log(`cd ${quoteShell(directRoot)} && ${quoteShell(aiwBinPath())} ${layoutArgs.map(quoteShell).join(" ")}`);
      return;
    }
    await runInherit(aiwBinPath(), layoutArgs, { cwd: directRoot });
    return;
  }

  if (selected) {
    const wtArgs = ["switch", selected, "-x", layoutCommand];
    if (flags.dryRun) {
      console.log(`cd ${quoteShell(repo)} && wt ${wtArgs.map(quoteShell).join(" ")}`);
      return;
    }
    await runInherit("wt", wtArgs, { cwd: repo });
    return;
  }

  const error = new Error("workspace target is required");
  error.exitCode = 2;
  throw error;
}

async function selectWorkspaceOpenTarget(repo, flags) {
  const workspaces = collectWorkspaceRecords(repo);
  const localBranches = listLocalBranches(repo);
  const worktreeBranches = new Set(workspaces.map((workspace) => workspace.branch).filter(Boolean));
  const entries = workspacePickerEntries(workspaces);

  if (!flags.worktreesOnly) {
    for (const branch of localBranches) {
      if (!worktreeBranches.has(branch)) {
        entries.push({
          label: pickerBranchLabel(branch, "branch", "no worktree"),
          target: branch
        });
      }
    }
  }

  if (flags.remotes) {
    const knownBranches = new Set([...localBranches, ...worktreeBranches]);
    for (const branch of listRemoteBranches(repo)) {
      if (!knownBranches.has(branch)) {
        entries.push({
          label: pickerBranchLabel(branch, "remote", "no local branch"),
          target: branch
        });
      }
    }
  }

  if (entries.length === 0) {
    const error = new Error("no workspaces or branches found");
    error.exitCode = 4;
    throw error;
  }

  const labels = entries.map((entry) => entry.label);
  const defaultEntry = entries.find((entry) => entry.previous) || entries.find((entry) => entry.current) || entries[0];
  const selected = await pickFromList("Open workspace", labels, {
    defaultItem: defaultEntry.label,
    force: true
  });
  const selectedKey = selected.trim();
  return entries.find((entry) => entry.label === selected || entry.label.trim() === selectedKey)?.target || selected;
}

function workspacePickerEntries(workspaces) {
  const branchWidth = Math.max(6, ...workspaces.map((workspace) => (workspace.branch || "(detached)").length));
  return workspaces.map((workspace) => {
    const branch = workspace.branch || "(detached)";
    const gitState = workspace.dirty ? "dirty" : "clean";
    const state = stateLabel(workspace.state);
    const cmux = workspace.cmux ? "open" : "-";
    const mark = workspace.current ? "@" : workspace.previous ? "-" : " ";
    return {
      label: `${mark} ${branch.padEnd(branchWidth)}  ${gitState.padEnd(5)}  ${state.padEnd(8)}  ${cmux.padEnd(4)}  ${workspace.path}`,
      target: workspace.branch || workspace.path,
      current: workspace.current,
      previous: workspace.previous
    };
  });
}

function pickerBranchLabel(branch, kind, note) {
  return `  ${branch.padEnd(Math.max(6, branch.length))}  ${kind.padEnd(5)}  ${"-".padEnd(8)}  ${"-".padEnd(4)}  ${note}`;
}

function pickerSwitchArgs(flags, layoutCommand) {
  const wtArgs = ["switch"];
  if (!flags.worktreesOnly) {
    wtArgs.push("--branches");
  }
  if (flags.remotes) {
    wtArgs.push("--remotes");
  }
  wtArgs.push("-x", layoutCommand);
  return wtArgs;
}

async function workspaceDone(config, argv) {
  assertGate("workspace", config);
  if (hasHelpFlag(argv)) {
    await runInherit("wt", ["merge", ...argv]);
    return;
  }
  const flags = parseDoneFlags(argv);
  const repo = assertGitRoot(process.cwd());
  assertDoneAllowed(repo);
  if (isDirty(repo)) {
    const error = new Error("working tree has uncommitted changes; use aiw git before aiw workspace done");
    error.exitCode = 5;
    throw error;
  }
  const closeTarget = flags.closeCmux ? cmuxWorkspaceRefForPath(repo) : "";
  const mergeArgs = await withSelectedMergeTarget(repo, flags.passthrough);
  const mergeEnv = worktrunkMergeEnv(config, repo, flags);
  await runWorkspaceHook(config, "pre_remove", {
    repo,
    cwd: repo,
    workspacePath: repo,
    branch: currentBranch(repo),
    target: mergeTarget(mergeArgs)
  });
  await runInherit("wt", ["merge", ...mergeArgs], { cwd: repo, env: mergeEnv });
  if (closeTarget) {
    closeCmuxWorkspace(closeTarget);
  }
}

async function workspaceRemove(config, argv) {
  assertGate("workspace", config);
  if (hasHelpFlag(argv)) {
    await runInherit("wt", ["remove", ...argv]);
    return;
  }
  const repo = assertGitRoot(process.cwd());
  const dirtyTargets = hasForceFlag(argv) ? [] : dirtyRemoveTargets(repo, argv);
  if (dirtyTargets.length > 0) {
    const error = new Error(`workspace has uncommitted changes: ${dirtyTargets.join(", ")}; rerun with --force only if you intend to discard/remove`);
    error.exitCode = 5;
    throw error;
  }
  for (const context of removeHookContexts(repo, argv)) {
    await runWorkspaceHook(config, "pre_remove", {
      ...context,
      dryRun: hasDryRunFlag(argv)
    });
  }
  await runInherit("wt", ["remove", ...argv], { cwd: repo });
}

async function workspaceGc(config, argv) {
  assertGate("workspace", config);
  const flags = parseWorkspaceFlags(argv);
  if (flags.dryRun && (flags.apply || flags.yes)) {
    const error = new Error("aiw workspace gc cannot combine --dry-run with --apply/--yes");
    error.exitCode = 2;
    throw error;
  }

  const repo = assertGitRoot(process.cwd());
  const staleSeconds = staleSecondsFromFlags(flags, config);
  const workspaces = collectWorkspaceRecords(repo);
  const plan = buildGcPlan(workspaces, {
    staleSeconds
  });
  if (flags.json && !flags.apply && !flags.yes) {
    console.log(JSON.stringify(plan, null, 2));
    return;
  }
  if (!flags.json) {
    console.log(formatGcPreview(plan, {
      color: shouldUseColor(flags),
      dryRun: flags.dryRun
    }));
  }
  if (flags.dryRun || plan.removable.length === 0) {
    if (flags.json && (flags.apply || flags.yes)) {
      console.log(JSON.stringify({
        ...plan,
        removed: [],
        skipped: []
      }, null, 2));
    }
    return;
  }

  if (!flags.apply && !flags.yes && !process.stdin.isTTY) {
    if (!flags.json) {
      console.log("Not interactive. Rerun with --apply or --yes to remove safe workspaces.");
    }
    return;
  }

  const shouldApply = flags.apply || flags.yes || await confirmGcApply(plan);
  if (!shouldApply) {
    if (!flags.json) {
      console.log("Cancelled. No files were removed.");
    }
    return;
  }

  const refreshedPlan = buildGcPlan(collectWorkspaceRecords(repo), {
    staleSeconds
  });
  const result = await applyGcPlan(config, repo, refreshedPlan);
  if (flags.json) {
    console.log(JSON.stringify({
      ...refreshedPlan,
      removed: result.removed,
      skipped: result.skipped
    }, null, 2));
    return;
  }
  if (result.removed.length === 0) {
    console.log("No removable workspaces after refresh.");
    return;
  }
  console.log(`Removed ${result.removed.length} workspace(s): ${result.removed.join(", ")}`);
}

export function collectWorkspaceRecords(repo) {
  const cmuxPaths = collectCmuxWorkspacePaths();
  const entries = readWorktrunkList(repo) || readGitWorktreeList(repo);
  const metadata = readWorkspaceMetadata(repo);
  const worktreeBranches = new Set(entries.map((entry) => entry.branch).filter(Boolean));
  const nonWorktreeBranches = listLocalBranches(repo).filter((branch) => !worktreeBranches.has(branch));
  return entries.map((entry) => {
    const resolvedPath = normalizePath(entry.path);
    const dirty = entry.dirty ?? isWorktreeDirty(resolvedPath);
    const lastChangedAt = entry.lastChangedAt || worktreeLastChangedAt(resolvedPath);
    const targetBranch = entry.branch ? stringValue(metadata.workspaces[entry.branch]?.targetBranch) : "";
    const targetMerged = entry.branch && targetBranch
      ? isBranchAncestor(repo, entry.branch, targetBranch)
      : false;
    const mergedTargets = entry.branch && !targetBranch
      ? mergedIntoTargets(repo, entry.branch, nonWorktreeBranches)
      : [];
    return {
      branch: entry.branch || "",
      path: resolvedPath,
      kind: entry.kind || "worktree",
      current: Boolean(entry.current),
      previous: Boolean(entry.previous),
      dirty,
      state: entry.state || "",
      integrationReason: entry.integrationReason || "",
      targetBranch,
      targetSource: targetBranch ? stringValue(metadata.workspaces[entry.branch]?.targetSource) || "aiw" : "",
      targetMerged,
      mergedTargets,
      cmux: cmuxPaths.has(resolvedPath),
      commit: entry.commit || "",
      lastChangedAt,
      ageSeconds: lastChangedAt ? Math.max(0, Math.floor((Date.now() - lastChangedAt) / 1000)) : null
    };
  });
}

function readWorktrunkList(repo) {
  const result = tryCapture("wt", ["list", "--format", "json"], { cwd: repo });
  if (!result.ok || !result.stdout) {
    return null;
  }
  try {
    const parsed = JSON.parse(result.stdout);
    if (!Array.isArray(parsed)) {
      return null;
    }
    return parsed.map((entry) => ({
      branch: stringValue(entry.branch),
      path: stringValue(entry.path),
      kind: stringValue(entry.kind),
      current: Boolean(entry.is_current),
      previous: Boolean(entry.is_previous),
      dirty: isWorktrunkDirty(entry.working_tree),
      state: stringValue(entry.main_state),
      integrationReason: stringValue(entry.integration_reason),
      commit: stringValue(entry.commit?.short_sha),
      lastChangedAt: timestampMillis(entry.commit?.timestamp)
    })).filter((entry) => entry.path);
  } catch {
    return null;
  }
}

function readGitWorktreeList(repo) {
  const result = tryCapture("git", ["worktree", "list", "--porcelain"], { cwd: repo });
  if (!result.ok || !result.stdout) {
    return [];
  }
  const current = normalizePath(repo);
  return result.stdout.split(/\n\n+/).map((block) => parseWorktreeBlock(block, current)).filter(Boolean);
}

function parseWorktreeBlock(block, current) {
  const entry = {
    branch: "",
    path: "",
    kind: "worktree",
    current: false,
    previous: false,
    commit: ""
  };
  for (const line of block.split(/\r?\n/)) {
    const [key, ...rest] = line.split(" ");
    const value = rest.join(" ");
    if (key === "worktree") {
      entry.path = value;
    } else if (key === "HEAD") {
      entry.commit = value.slice(0, 7);
    } else if (key === "branch") {
      entry.branch = value.startsWith("refs/heads/") ? value.slice("refs/heads/".length) : value;
    } else if (key === "detached") {
      entry.kind = "detached";
    } else if (key === "bare") {
      entry.kind = "bare";
    }
  }
  if (!entry.path) {
    return null;
  }
  entry.current = normalizePath(entry.path) === current;
  return entry;
}

function collectCmuxWorkspacePaths() {
  const paths = new Set();
  const result = tryCapture("cmux", ["list-workspaces", "--json"]);
  if (!result.ok || !result.stdout) {
    return paths;
  }
  try {
    const parsed = JSON.parse(result.stdout);
    const workspaces = Array.isArray(parsed.workspaces) ? parsed.workspaces : [];
    for (const workspace of workspaces) {
      if (workspace.current_directory) {
        paths.add(normalizePath(workspace.current_directory));
      }
    }
  } catch {
    return paths;
  }
  return paths;
}

function cmuxWorkspaceRefForPath(workspacePath) {
  const targetPath = normalizePath(workspacePath);
  const result = tryCapture("cmux", ["list-workspaces", "--json"]);
  if (!result.ok || !result.stdout) {
    return "";
  }
  try {
    const parsed = JSON.parse(result.stdout);
    const workspaces = Array.isArray(parsed.workspaces) ? parsed.workspaces : [];
    const match = workspaces.find((workspace) => {
      return workspace.current_directory && normalizePath(workspace.current_directory) === targetPath;
    });
    return stringValue(match?.ref);
  } catch {
    return "";
  }
}

function closeCmuxWorkspace(workspaceRef) {
  if (!workspaceRef) {
    return;
  }
  tryCapture("cmux", ["close-workspace", "--workspace", workspaceRef]);
}

function assertDoneAllowed(repo) {
  const current = currentWorkspaceRecord(repo);
  if (current?.state === "is_main" || isPrimaryWorktree(repo)) {
    const error = new Error("aiw workspace done must be run from a feature worktree, not the main workspace");
    error.exitCode = 5;
    throw error;
  }
}

function currentWorkspaceRecord(repo) {
  const currentPath = normalizePath(repo);
  return collectWorkspaceRecords(repo).find((workspace) => {
    return workspace.current || workspace.path === currentPath;
  });
}

function isPrimaryWorktree(repo) {
  const gitDir = gitRevPath(repo, "--git-dir");
  const commonDir = gitRevPath(repo, "--git-common-dir");
  return Boolean(gitDir && commonDir && gitDir === commonDir);
}

function gitRevPath(repo, option) {
  const result = tryCapture("git", ["rev-parse", option], { cwd: repo });
  if (!result.ok || !result.stdout) {
    return "";
  }
  const resolved = path.isAbsolute(result.stdout)
    ? result.stdout
    : path.resolve(repo, result.stdout);
  return normalizePath(resolved);
}

function formatWorkspaceTable(workspaces, options = {}) {
  if (workspaces.length === 0) {
    return "No workspaces found.";
  }
  const painter = createPainter(options.color);
  const plan = buildGcPlan(workspaces, {
    staleSeconds: options.staleSeconds ?? DEFAULT_STALE_SECONDS
  });
  const signals = new Map([
    ...plan.removable.map((workspace) => [workspace.path, workspace.gc]),
    ...plan.warnings.map((workspace) => [workspace.path, workspace.gc])
  ]);
  const rows = workspaces.map((workspace) => ({
    workspace: {
      ...workspace,
      gc: signals.get(workspace.path) || enrichWorkspaceSignals(workspace, plan.staleSeconds).gc
    },
    values: {
      mark: workspace.current ? "@" : workspace.previous ? "-" : "",
      branch: workspace.branch || "(detached)",
      git: workspace.dirty ? "dirty" : "clean",
      state: stateLabel(workspace.state),
      target: targetLabel(workspace),
      merged: mergedLabel(workspace),
      cmux: workspace.cmux ? "open" : "-",
      age: formatAge(workspace.ageSeconds),
      gc: gcLabel(signals.get(workspace.path) || enrichWorkspaceSignals(workspace, plan.staleSeconds).gc),
      path: workspace.path
    }
  }));
  const columns = [
    ["", "mark"],
    ["BRANCH", "branch"],
    ["GIT", "git"],
    ["STATE", "state"],
    ["TARGET", "target"],
    ["MERGED", "merged"],
    ["CMUX", "cmux"],
    ["AGE", "age"],
    ["GC", "gc"],
    ["PATH", "path"]
  ];
  const widths = columns.map(([header, key]) => {
    return Math.max(header.length, ...rows.map((row) => String(row.values[key]).length));
  });
  const summary = formatWorkspaceSummary(workspaces, plan, painter);
  const header = painter.bold(columns.map(([label], index) => label.padEnd(widths[index])).join("  ").trimEnd());
  const body = rows.map((row) => {
    return columns.map(([, key], index) => {
      const value = String(row.values[key]);
      return colorWorkspaceCell(key, value.padEnd(widths[index]), row.workspace, painter);
    }).join("  ").trimEnd();
  });
  return [
    summary,
    header,
    ...body,
    formatStateLegend(workspaces, painter)
  ].join("\n");
}

function findWorkspace(workspaces, target) {
  const normalizedTarget = normalizePath(target);
  return workspaces.find((workspace) => {
    return workspace.branch === target ||
      workspace.path === normalizedTarget ||
      path.basename(workspace.path) === target;
  });
}

function gitRootIfExists(target) {
  const resolved = normalizePath(target);
  if (!resolved || !fs.existsSync(resolved)) {
    return "";
  }
  const result = tryCapture("git", ["rev-parse", "--show-toplevel"], { cwd: resolved });
  return result.ok ? normalizePath(result.stdout) : "";
}

function listLocalBranches(repo) {
  const result = tryCapture("git", ["for-each-ref", "--format=%(refname:short)", "refs/heads"], { cwd: repo });
  return result.ok ? result.stdout.split(/\r?\n/).filter(Boolean).sort() : [];
}

function listRemoteBranches(repo) {
  const result = tryCapture("git", ["for-each-ref", "--format=%(refname:short)", "refs/remotes"], { cwd: repo });
  if (!result.ok) {
    return [];
  }
  return [...new Set(result.stdout.split(/\r?\n/)
    .filter((branch) => branch && !branch.endsWith("/HEAD"))
    .map((branch) => branch.startsWith("origin/") ? branch.slice("origin/".length) : branch))]
    .sort();
}

function currentBranch(repo) {
  const result = tryCapture("git", ["branch", "--show-current"], { cwd: repo });
  return result.ok ? result.stdout : "";
}

function mergedIntoTargets(repo, branch, targets) {
  return targets.filter((target) => {
    if (!target || target === branch) {
      return false;
    }
    return isBranchAncestor(repo, branch, target);
  });
}

function isBranchAncestor(repo, branch, target) {
  const result = tryCapture("git", ["merge-base", "--is-ancestor", branch, target], { cwd: repo });
  return result.ok;
}

function isWorktreeDirty(worktreePath) {
  if (!worktreePath) {
    return false;
  }
  const result = tryCapture("git", ["status", "--short"], { cwd: worktreePath });
  return result.ok && result.stdout.length > 0;
}

function worktreeLastChangedAt(worktreePath) {
  if (!worktreePath) {
    return null;
  }
  const result = tryCapture("git", ["log", "-1", "--format=%ct"], { cwd: worktreePath });
  return result.ok ? timestampMillis(result.stdout) : null;
}

function timestampMillis(value) {
  const timestamp = Number(value);
  return Number.isFinite(timestamp) && timestamp > 0 ? timestamp * 1000 : null;
}

function isWorktrunkDirty(workingTree) {
  if (!workingTree || typeof workingTree !== "object") {
    return undefined;
  }
  return Boolean(
    workingTree.staged ||
    workingTree.modified ||
    workingTree.untracked ||
    workingTree.renamed ||
    workingTree.deleted
  );
}

function buildGcPlan(workspaces, options = {}) {
  const staleSeconds = options.staleSeconds ?? DEFAULT_STALE_SECONDS;
  const enriched = workspaces.map((workspace) => enrichWorkspaceSignals(workspace, staleSeconds));
  const removable = enriched.filter((workspace) => workspace.gc.removable).map((workspace) => ({
    ...workspace,
    command: removeCommand(workspace)
  }));
  const warnings = enriched.filter((workspace) => workspace.gc.stale && !workspace.gc.removable).map((workspace) => ({
    ...workspace,
    reason: gcBlockedReason(workspace)
  }));
  return {
    staleSeconds,
    removable,
    warnings
  };
}

function enrichWorkspaceSignals(workspace, staleSeconds) {
  const merged = isMergedWorkspace(workspace);
  const stale = typeof workspace.ageSeconds === "number" && workspace.ageSeconds >= staleSeconds;
  const removable = !workspace.current && !workspace.dirty && merged && Boolean(workspace.branch || workspace.path);
  return {
    ...workspace,
    gc: {
      merged,
      stale,
      removable
    }
  };
}

function isMergedWorkspace(workspace) {
  if (workspace.targetBranch) {
    return Boolean(workspace.targetMerged);
  }
  return INTEGRATED_STATES.has(workspace.state) || (Array.isArray(workspace.mergedTargets) && workspace.mergedTargets.length > 0);
}

function gcBlockedReason(workspace) {
  const reasons = [];
  if (workspace.current) {
    reasons.push("current");
  }
  if (workspace.dirty) {
    reasons.push("dirty");
  }
  if (!workspace.gc?.merged) {
    reasons.push("not merged");
  }
  if (!workspace.branch && !workspace.path) {
    reasons.push("missing target");
  }
  return reasons.join(", ") || "manual review";
}

function formatGcPreview(plan, options = {}) {
  const painter = createPainter(options.color);
  const sections = [
    `${options.dryRun ? "GC dry-run" : "GC plan"}: ${painter.green(String(plan.removable.length))} removable, ${plan.warnings.length > 0 ? painter.yellow(String(plan.warnings.length)) : "0"} stale warning(s). stale >= ${plan.staleSeconds}s`
  ];

  if (plan.removable.length > 0) {
    sections.push(formatGcTable("Removable: clean + merged/integrated", plan.removable, painter, true));
  } else {
    sections.push("Removable: none");
  }

  if (plan.warnings.length > 0) {
    sections.push(formatGcTable("Warnings: stale but blocked from auto cleanup", plan.warnings, painter, false));
  }

  sections.push(options.dryRun
    ? "No files were removed. Run without --dry-run to confirm cleanup, or use --apply/--yes."
    : "Only removable workspaces can be deleted; stale warnings are not touched.");
  return sections.join("\n");
}

function formatGcTable(title, workspaces, painter, includeCommand) {
  const rows = workspaces.map((workspace) => ({
    workspace,
    values: {
      branch: workspace.branch || "(detached)",
      age: formatAge(workspace.ageSeconds),
      state: stateLabel(workspace.state),
      path: workspace.path,
      reason: workspace.reason || "-",
      command: workspace.command || removeCommand(workspace)
    }
  }));
  const columns = includeCommand ? [
    ["BRANCH", "branch"],
    ["AGE", "age"],
    ["STATE", "state"],
    ["PATH", "path"],
    ["COMMAND", "command"]
  ] : [
    ["BRANCH", "branch"],
    ["AGE", "age"],
    ["STATE", "state"],
    ["REASON", "reason"],
    ["PATH", "path"]
  ];
  const widths = columns.map(([header, key]) => {
    return Math.max(header.length, ...rows.map((row) => String(row.values[key]).length));
  });
  const header = painter.bold(columns.map(([label], index) => label.padEnd(widths[index])).join("  ").trimEnd());
  const body = rows.map((row) => {
    return columns.map(([, key], index) => {
      const value = String(row.values[key]).padEnd(widths[index]);
      if (key === "state") {
        return colorState(value, row.workspace.state, painter);
      }
      if (key === "command") {
        return painter.dim(value);
      }
      if (key === "reason") {
        return painter.yellow(value);
      }
      return value;
    }).join("  ").trimEnd();
  });
  return [title, header, ...body].join("\n");
}

async function confirmGcApply(plan) {
  if (!process.stdin.isTTY) {
    return false;
  }
  const answer = await askInput(`Remove ${plan.removable.length} safe workspace(s)? Type y to confirm`);
  return answer.toLowerCase() === "y";
}

async function applyGcPlan(config, repo, plan) {
  const removed = [];
  const skipped = [];
  for (const workspace of plan.removable) {
    const target = workspace.branch || workspace.path;
    if (!target) {
      skipped.push({
        target: workspace.path || workspace.branch || "",
        reason: "missing target"
      });
      continue;
    }
    try {
      await runWorkspaceHook(config, "pre_remove", {
        repo: workspace.path || repo,
        cwd: workspace.path || repo,
        workspacePath: workspace.path || "",
        branch: workspace.branch || "",
        target
      });
    } catch (error) {
      skipped.push({
        target,
        reason: error.message
      });
      continue;
    }
    const result = tryCapture("wt", ["--yes", "remove", target], { cwd: repo });
    if (!result.ok) {
      skipped.push({
        target,
        reason: result.stderr || result.stdout || `wt remove exited with ${result.status}`
      });
      continue;
    }
    removed.push(target);
    const workspaceRef = cmuxWorkspaceRefForPath(workspace.path);
    if (workspaceRef) {
      closeCmuxWorkspace(workspaceRef);
    }
  }
  return {
    removed,
    skipped
  };
}

function removeCommand(workspace) {
  const target = workspace.branch || workspace.path;
  return `aiw workspace remove ${quoteShell(target)}`;
}

function targetLabel(workspace) {
  if (workspace.targetBranch) {
    return workspace.targetBranch;
  }
  if (Array.isArray(workspace.mergedTargets) && workspace.mergedTargets.length > 0) {
    return `?${workspace.mergedTargets[0]}`;
  }
  return "-";
}

function mergedLabel(workspace) {
  if (workspace.targetBranch) {
    return workspace.targetMerged ? "yes" : "no";
  }
  if (INTEGRATED_STATES.has(workspace.state)) {
    return "inferred";
  }
  if (Array.isArray(workspace.mergedTargets) && workspace.mergedTargets.length > 0) {
    return "inferred";
  }
  return "-";
}

function gcLabel(gc) {
  if (gc?.removable) {
    return "remove";
  }
  if (gc?.stale) {
    return "stale";
  }
  return "-";
}

function formatAge(ageSeconds) {
  if (typeof ageSeconds !== "number") {
    return "-";
  }
  if (ageSeconds < 60) {
    return `${ageSeconds}s`;
  }
  const minutes = Math.floor(ageSeconds / 60);
  if (minutes < 60) {
    return `${minutes}m`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 48) {
    return `${hours}h`;
  }
  return `${Math.floor(hours / 24)}d`;
}

function formatWorkspaceSummary(workspaces, plan, painter) {
  const dirty = workspaces.filter((workspace) => workspace.dirty).length;
  const open = workspaces.filter((workspace) => workspace.cmux).length;
  const gc = plan.removable.length;
  const stale = plan.warnings.length;
  return [
    painter.bold(`Workspaces: ${workspaces.length}`),
    `dirty ${dirty > 0 ? painter.red(String(dirty)) : painter.green("0")}`,
    `cmux open ${open > 0 ? painter.green(String(open)) : "0"}`,
    `removable ${gc > 0 ? painter.green(String(gc)) : "0"}`,
    `stale warnings ${stale > 0 ? painter.yellow(String(stale)) : "0"}`
  ].join("  ");
}

function colorWorkspaceCell(key, value, workspace, painter) {
  if (key === "mark" && workspace.current) {
    return painter.cyan(painter.bold(value));
  }
  if (key === "mark" && workspace.previous) {
    return painter.yellow(value);
  }
  if (key === "branch" && workspace.current) {
    return painter.bold(value);
  }
  if (key === "git") {
    return workspace.dirty ? painter.red(painter.bold(value)) : painter.green(value);
  }
  if (key === "state") {
    return colorState(value, workspace.state, painter);
  }
  if (key === "target") {
    if (workspace.targetBranch) {
      return painter.cyan(value);
    }
    return value.trim().startsWith("?") ? painter.yellow(value) : painter.dim(value);
  }
  if (key === "merged") {
    if (value.trim() === "yes") {
      return painter.green(value);
    }
    if (value.trim() === "inferred") {
      return painter.yellow(value);
    }
    return value.trim() === "-" ? painter.dim(value) : painter.red(value);
  }
  if (key === "cmux") {
    return workspace.cmux ? painter.green(value) : painter.dim(value);
  }
  if (key === "gc") {
    if (workspace.gc?.removable) {
      return painter.green(value);
    }
    if (workspace.gc?.stale) {
      return painter.yellow(value);
    }
    return painter.dim(value);
  }
  if (key === "age" && workspace.gc?.stale) {
    return painter.yellow(value);
  }
  return value;
}

function colorState(value, state, painter) {
  if (state === "is_main") {
    return painter.cyan(value);
  }
  if (INTEGRATED_STATES.has(state)) {
    return painter.green(value);
  }
  if (state === "would_conflict" || state === "orphan") {
    return painter.red(painter.bold(value));
  }
  if (state === "diverged") {
    return painter.magenta(value);
  }
  if (state === "ahead" || state === "behind") {
    return painter.yellow(value);
  }
  return painter.dim(value);
}

function formatStateLegend(workspaces, painter) {
  const states = [...new Set(workspaces.map((workspace) => workspace.state).filter(Boolean))];
  const shown = states.map((state) => {
    return `${colorState(stateLabel(state), state, painter)}=${stateDescription(state)}`;
  });
  if (shown.length === 0) {
    return `STATE: ${painter.dim("no state values")}  GC: remove=clean+merged, stale=old but manual review. Full legend: aiw workspace states`;
  }
  return `STATE: ${shown.join("; ")}. GC: remove=clean+merged, stale=old but manual review. Full legend: aiw workspace states`;
}

function stateLabel(state) {
  switch (state) {
    case "is_main":
      return "main";
    case "same_commit":
      return "same";
    case "integrated":
      return "merged";
    case "would_conflict":
      return "conflict";
    case "":
      return "-";
    default:
      return state;
  }
}

function stateDescription(state) {
  switch (state) {
    case "is_main":
      return "default worktree";
    case "orphan":
      return "no common base with default";
    case "would_conflict":
      return "merge would conflict";
    case "empty":
      return "no effective branch changes";
    case "same_commit":
      return "same HEAD as default";
    case "integrated":
      return "content already in default";
    case "diverged":
      return "both branch and default moved";
    case "ahead":
      return "branch has changes to merge";
    case "behind":
      return "behind default";
    default:
      return state || "unknown";
  }
}

function createPainter(enabled) {
  const wrap = (code, text) => enabled ? `${code}${text}${ANSI.reset}` : text;
  return {
    bold: (text) => wrap(ANSI.bold, text),
    dim: (text) => wrap(ANSI.dim, text),
    red: (text) => wrap(ANSI.red, text),
    green: (text) => wrap(ANSI.green, text),
    yellow: (text) => wrap(ANSI.yellow, text),
    magenta: (text) => wrap(ANSI.magenta, text),
    cyan: (text) => wrap(ANSI.cyan, text),
    gray: (text) => wrap(ANSI.gray, text)
  };
}

function shouldUseColor(flags) {
  if (flags.color === "always") {
    return true;
  }
  if (flags.color === "never" || process.env.NO_COLOR) {
    return false;
  }
  return process.stdout.isTTY;
}

function hasForceFlag(argv) {
  return argv.includes("--force") || argv.includes("-f");
}

function hasHelpFlag(argv) {
  return argv.includes("--help") || argv.includes("-h");
}

function hasDryRunFlag(argv) {
  return argv.includes("--dry-run");
}

function worktrunkMergeEnv(config, repo, flags) {
  if (!mergeNeedsCommitGeneration(flags.passthrough)) {
    return process.env;
  }
  if (process.env.WORKTRUNK_COMMIT__GENERATION__COMMAND) {
    return process.env;
  }
  if (worktrunkCommitGenerationConfigured(repo, flags.passthrough)) {
    return process.env;
  }

  const agent = resolveAgent(config, flags.agent || config.commit.agent || config.defaults.agent);
  assertGate("commit", config, agent);
  const command = `${quoteShell(aiwBinPath())} commit-message --agent ${quoteShell(agent.name)}`;
  return {
    ...process.env,
    WORKTRUNK_COMMIT__GENERATION__COMMAND: command
  };
}

function mergeNeedsCommitGeneration(argv) {
  return !argv.includes("--no-squash") && !argv.includes("--no-commit");
}

function worktrunkCommitGenerationConfigured(repo, argv) {
  const result = tryCapture("wt", ["config", "show", "--format", "json", ...worktrunkConfigArgs(argv)], { cwd: repo });
  if (!result.ok || !result.stdout) {
    return false;
  }
  try {
    const config = JSON.parse(result.stdout);
    return Boolean(
      commitGenerationCommand(config.user?.config) ||
      commitGenerationCommand(config.project?.config) ||
      commitGenerationCommand(config.system?.config)
    );
  } catch {
    return false;
  }
}

function worktrunkConfigArgs(argv) {
  const args = [];
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--config" && argv[index + 1]) {
      args.push("--config", argv[index + 1]);
      index += 1;
    } else if (arg.startsWith("--config=")) {
      args.push("--config", arg.slice("--config=".length));
    }
  }
  return args;
}

function commitGenerationCommand(config) {
  if (!config || typeof config !== "object") {
    return "";
  }
  return stringValue(config.commit?.generation?.command) ||
    stringValue(config["commit.generation"]?.command) ||
    stringValue(config["commit.generation.command"]);
}

async function withSelectedMergeTarget(repo, argv) {
  if (mergeTarget(argv)) {
    return argv;
  }
  const current = currentBranch(repo);
  const recordedTarget = workspaceTargetBranch(repo, current);
  if (!process.stdin.isTTY) {
    return recordedTarget ? [...argv, recordedTarget] : argv;
  }
  const branches = listLocalBranches(repo).filter((branch) => branch !== current);
  if (recordedTarget && !branches.includes(recordedTarget)) {
    branches.unshift(recordedTarget);
  }
  if (branches.length === 0) {
    return argv;
  }
  const selected = await pickFromList("Merge target", branches, {
    defaultItem: recordedTarget || defaultMergeTarget(branches)
  });
  return [...argv, selected];
}

function mergeTarget(argv) {
  const optionsWithValues = new Set(["-C", "--config", "--format", "--stage"]);
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (optionsWithValues.has(arg)) {
      index += 1;
      continue;
    }
    if (arg.startsWith("-")) {
      continue;
    }
    return arg;
  }
  return "";
}

function defaultMergeTarget(branches) {
  return branches.includes("dev")
    ? "dev"
    : branches.find((branch) => branch === "develop") || branches[0];
}

function dirtyRemoveTargets(repo, argv) {
  const targets = removeTargets(argv);
  if (targets.length === 0) {
    return isDirty(repo) ? [repo] : [];
  }

  const workspaces = collectWorkspaceRecords(repo);
  const dirtyPaths = [];
  for (const target of targets) {
    const record = findWorkspace(workspaces, target);
    const targetRoot = record?.path || gitRootIfExists(target);
    if (targetRoot && isDirty(targetRoot)) {
      dirtyPaths.push(targetRoot);
    }
  }
  return [...new Set(dirtyPaths)];
}

function removeHookContexts(repo, argv) {
  const targets = removeTargets(argv);
  if (targets.length === 0) {
    return [{
      repo,
      cwd: repo,
      workspacePath: repo,
      branch: currentBranch(repo),
      target: repo
    }];
  }

  const workspaces = collectWorkspaceRecords(repo);
  const contexts = [];
  const seen = new Set();
  for (const target of targets) {
    const record = findWorkspace(workspaces, target);
    const workspacePath = record?.path || gitRootIfExists(target) || repo;
    const key = `${workspacePath}\0${target}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    contexts.push({
      repo: workspacePath,
      cwd: workspacePath,
      workspacePath,
      branch: record?.branch || "",
      target
    });
  }
  return contexts;
}

function removeTargets(argv) {
  const targets = [];
  const optionsWithValues = new Set(["-C", "--config", "--format"]);
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--") {
      targets.push(...argv.slice(index + 1));
      break;
    }
    if (optionsWithValues.has(arg)) {
      index += 1;
      continue;
    }
    if (arg.startsWith("-")) {
      continue;
    }
    targets.push(arg);
  }
  return targets;
}

function normalizePath(value) {
  if (!value) {
    return "";
  }
  const expanded = value === "~" || value.startsWith("~/")
    ? path.join(process.env.HOME || "", value.slice(value === "~" ? 1 : 2))
    : value;
  const resolved = path.resolve(expanded);
  try {
    return fs.realpathSync.native(resolved);
  } catch {
    return resolved;
  }
}

function stringValue(value) {
  return typeof value === "string" ? value : "";
}

function workspaceTargetBranch(repo, branch) {
  return branch ? stringValue(readWorkspaceMetadata(repo).workspaces[branch]?.targetBranch) : "";
}

function readWorkspaceMetadata(repo) {
  const metadataPath = workspaceMetadataPath(repo);
  if (!metadataPath || !fs.existsSync(metadataPath)) {
    return emptyWorkspaceMetadata();
  }
  try {
    const parsed = JSON.parse(fs.readFileSync(metadataPath, "utf8"));
    return {
      version: 1,
      workspaces: parsed && typeof parsed.workspaces === "object" && parsed.workspaces !== null
        ? parsed.workspaces
        : {}
    };
  } catch {
    return emptyWorkspaceMetadata();
  }
}

function writeWorkspaceMetadata(repo, metadata) {
  const metadataPath = workspaceMetadataPath(repo);
  if (!metadataPath) {
    return;
  }
  fs.mkdirSync(path.dirname(metadataPath), { recursive: true });
  fs.writeFileSync(metadataPath, `${JSON.stringify({
    version: 1,
    workspaces: metadata.workspaces || {}
  }, null, 2)}\n`);
}

function emptyWorkspaceMetadata() {
  return {
    version: 1,
    workspaces: {}
  };
}

function workspaceMetadataPath(repo) {
  const result = tryCapture("git", ["rev-parse", "--git-common-dir"], { cwd: repo });
  if (!result.ok || !result.stdout) {
    return "";
  }
  const commonDir = path.isAbsolute(result.stdout)
    ? result.stdout
    : path.resolve(repo, result.stdout);
  return path.join(commonDir, "aiw", "workspaces.json");
}

function parseWorkspaceFlags(argv) {
  const flags = {
    positionals: []
  };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    switch (arg) {
      case "--agent":
        flags.agent = argv[++index];
        break;
      case "--json":
        flags.json = true;
        break;
      case "--dry-run":
        flags.dryRun = true;
        break;
      case "--stale-seconds":
        flags.staleSeconds = Number(argv[++index]);
        break;
      case "--apply":
        flags.apply = true;
        break;
      case "--yes":
      case "-y":
        flags.yes = true;
        break;
      case "--no-close-cmux":
        flags.closeCmux = false;
        break;
      case "--close-cmux":
        flags.closeCmux = true;
        break;
      case "--branches":
        flags.branches = true;
        break;
      case "--remotes":
        flags.remotes = true;
        break;
      case "--worktrees-only":
        flags.worktreesOnly = true;
        break;
      case "--no-color":
        flags.color = "never";
        break;
      case "--color": {
        const value = argv[++index] || "always";
        flags.color = value === "never" || value === "auto" ? value : "always";
        break;
      }
      default:
        flags.positionals.push(arg);
    }
  }
  return flags;
}

function parseDoneFlags(argv) {
  const flags = {
    closeCmux: true,
    agent: "",
    passthrough: []
  };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--no-close-cmux") {
      flags.closeCmux = false;
    } else if (arg === "--close-cmux") {
      flags.closeCmux = true;
    } else if (arg === "--agent") {
      flags.agent = argv[++index] || "";
    } else if (arg.startsWith("--agent=")) {
      flags.agent = arg.slice("--agent=".length);
    } else {
      flags.passthrough.push(arg);
    }
  }
  return flags;
}

function staleSecondsFromFlags(flags, config = {}) {
  if (flags.staleSeconds === undefined) {
    const configured = Number(config.workspace?.stale_seconds);
    return Number.isFinite(configured) && configured >= 0
      ? Math.floor(configured)
      : DEFAULT_STALE_SECONDS;
  }
  if (!Number.isFinite(flags.staleSeconds) || flags.staleSeconds < 0) {
    const error = new Error("--stale-seconds must be a non-negative number");
    error.exitCode = 2;
    throw error;
  }
  return Math.floor(flags.staleSeconds);
}

function printStateHelp() {
  console.log(`Workspace state values:
  is_main         Default branch worktree
  empty           No effective branch changes
  same_commit     Branch HEAD equals default branch HEAD
  integrated      Branch content is already integrated into default
  ahead           Branch has changes to merge into default
  behind          Branch is behind default
  diverged        Branch and default both moved
  would_conflict  Simulated merge would conflict
  orphan          No common ancestor with default

Integration reasons when state is integrated:
  ancestor             Branch is in default branch history
  trees_match          Branch tree content matches default
  no_added_changes     Diff from default adds no changes
  merge_adds_nothing   Simulated merge produces default tree
  patch-id-match       Branch diff matches a squash-merge commit`);
}

function printWorkspaceHelp() {
  console.log(`Usage: aiw workspace <command> [options]

Commands:
  list [--json] [--color mode] [--stale-seconds n]
                                               List worktrees with dirty, age, GC, and cmux status
  status [--json]                             Alias for list
  open [target] [--agent name] [--remotes]    Open picker or target with the AIW cmux layout
  switch [target]                             Alias for open
  done [target] [--agent name] [--no-close-cmux]
                                               Merge the current feature worktree, cleanup, then close cmux workspace
  remove [wt-remove-args...]                  Remove worktrees after dirty check
  gc|clean [--dry-run] [--apply|--yes] [--json] [--stale-seconds n]
                                               Preview or remove safe worktrees; stale warnings are not removed
  states                                      Explain workspace state values

Short aliases:
  aiw ws list                   aiw workspace list
  aiw list                      aiw workspace list
  aiw ws open [target]          aiw workspace open [target]
  aiw open [target]             aiw workspace open [target]
  aiw switch [target]           aiw workspace open [target]
  aiw done                      aiw workspace done
  aiw remove                    aiw workspace remove
  aiw gc                        aiw workspace gc
  aiw clean                     aiw workspace gc`);
}
