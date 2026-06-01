import fs from "node:fs";
import path from "node:path";
import { expandHome, parseToml } from "./config.mjs";
import { quoteShell, runInherit, tryCapture } from "./run.mjs";

const PROJECT_CONFIG = ".aiw.toml";

export function workspaceHookPlan(config, event, context = {}) {
  const repo = context.repo || context.cwd || process.cwd();
  const cwd = context.cwd || repo;
  const project = projectInfo(repo);
  const sources = [
    {
      kind: "global",
      path: config.aiwPath,
      workspace: config.workspace || {}
    },
    ...globalProjectHookSources(config.workspace || {}, config.aiwPath, project),
    projectHookSource(repo, config.aiwPath)
  ].filter(Boolean);

  const commands = [];
  for (const source of sources) {
    for (const command of hookCommands(source.workspace, event, source.path)) {
      commands.push({
        event,
        command,
        cwd,
        source: source.kind,
        configPath: source.path,
        project,
        rule: source.rule || ""
      });
    }
  }
  return commands;
}

export async function runWorkspaceHook(config, event, context = {}) {
  const plan = workspaceHookPlan(config, event, context);
  if (plan.length === 0) {
    return;
  }
  if (context.dryRun) {
    printWorkspaceHookPlan(plan);
    return;
  }
  for (const item of plan) {
    const label = item.rule ? `${item.source}/${item.rule}` : item.source;
    console.log(`[aiw hook:${event}] ${label} ${item.command}`);
    try {
      await runInherit("sh", ["-lc", item.command], {
        cwd: item.cwd,
        env: hookEnv(item, context)
      });
    } catch (error) {
      error.message = `workspace hook '${event}' failed from ${item.configPath}: ${error.message}`;
      throw error;
    }
  }
}

export function printWorkspaceHookPlan(plan) {
  for (const item of plan) {
    const label = item.rule ? `${item.source}/${item.rule}` : item.source;
    console.log(`# hook ${item.event} (${label}: ${item.configPath})`);
    console.log(`cd ${quoteShell(item.cwd)} && sh -lc ${quoteShell(item.command)}`);
  }
}

function globalProjectHookSources(workspace, configPath, project) {
  const projects = workspace?.hooks?.projects;
  if (projects === undefined) {
    return [];
  }
  if (!projects || typeof projects !== "object" || Array.isArray(projects)) {
    const error = new Error(`workspace hook projects in ${configPath} must be a table`);
    error.exitCode = 2;
    throw error;
  }
  return Object.entries(projects)
    .filter(([name, entry]) => projectHookEntryMatches(name, entry, project, configPath))
    .map(([name, entry]) => ({
      kind: "global-project",
      rule: name,
      path: configPath,
      workspace: entry
    }));
}

function projectHookSource(repo, globalConfigPath) {
  const configPath = path.join(repo, PROJECT_CONFIG);
  if (configPath === globalConfigPath || !fs.existsSync(configPath)) {
    return null;
  }
  const parsed = parseToml(fs.readFileSync(configPath, "utf8"));
  return {
    kind: "project",
    path: configPath,
    workspace: parsed.workspace || {}
  };
}

function projectHookEntryMatches(name, entry, project, sourcePath) {
  if (!entry || typeof entry !== "object" || Array.isArray(entry)) {
    const error = new Error(`workspace project hook entry in ${sourcePath} must be a table`);
    error.exitCode = 2;
    throw error;
  }
  const namePatterns = [
    ...stringList(entry.match, "match", sourcePath),
    ...stringList(entry.matches, "matches", sourcePath),
    ...stringList(entry.project, "project", sourcePath),
    ...stringList(entry.projects, "projects", sourcePath)
  ];
  const pathPatterns = [
    ...stringList(entry.path, "path", sourcePath),
    ...stringList(entry.paths, "paths", sourcePath),
    ...stringList(entry.repo, "repo", sourcePath),
    ...stringList(entry.repos, "repos", sourcePath)
  ];
  const effectiveNamePatterns = namePatterns.length > 0 ? namePatterns : [name];
  return effectiveNamePatterns.some((pattern) => matchProjectPattern(pattern, project)) ||
    pathPatterns.some((pattern) => matchProjectPath(pattern, project));
}

function hookCommands(workspace, event, sourcePath) {
  const hooks = workspace?.hooks || {};
  const raw = hooks[event] ?? workspace[event];
  if (raw === undefined) {
    return [];
  }
  if (typeof raw === "string") {
    return raw.trim() ? [raw] : [];
  }
  if (Array.isArray(raw) && raw.every((item) => typeof item === "string")) {
    return raw.map((item) => item.trim()).filter(Boolean);
  }
  const error = new Error(`workspace hook '${event}' in ${sourcePath} must be a string or an array of strings`);
  error.exitCode = 2;
  throw error;
}

function hookEnv(item, context) {
  return {
    ...process.env,
    AIW_HOOK_EVENT: item.event,
    AIW_HOOK_SOURCE: item.source,
    AIW_HOOK_CONFIG: item.configPath,
    AIW_HOOK_CWD: item.cwd,
    AIW_HOOK_RULE: item.rule || "",
    AIW_REPO: context.repo || "",
    AIW_PROJECT_PATH: item.project.commonPath || item.project.currentPath || "",
    AIW_PROJECT_NAME: item.project.commonName || item.project.currentName || "",
    AIW_WORKSPACE_PATH: context.workspacePath || item.cwd,
    AIW_WORKSPACE_BRANCH: context.branch || "",
    AIW_WORKSPACE_TARGET: context.target || context.branch || context.workspacePath || "",
    AIW_AGENT: context.agent || ""
  };
}

function stringList(value, key, sourcePath) {
  if (value === undefined) {
    return [];
  }
  if (typeof value === "string") {
    return value.trim() ? [value.trim()] : [];
  }
  if (Array.isArray(value) && value.every((item) => typeof item === "string")) {
    return value.map((item) => item.trim()).filter(Boolean);
  }
  const error = new Error(`workspace project hook '${key}' in ${sourcePath} must be a string or an array of strings`);
  error.exitCode = 2;
  throw error;
}

function projectInfo(repo) {
  const currentPath = normalizePath(repo);
  const commonPath = gitCommonRoot(currentPath) || currentPath;
  return {
    currentPath,
    currentName: path.basename(currentPath),
    commonPath,
    commonName: path.basename(commonPath)
  };
}

function gitCommonRoot(repo) {
  const result = tryCapture("git", ["rev-parse", "--git-common-dir"], { cwd: repo });
  if (!result.ok || !result.stdout) {
    return "";
  }
  const commonDir = normalizePath(path.isAbsolute(result.stdout)
    ? result.stdout
    : path.resolve(repo, result.stdout));
  return path.basename(commonDir) === ".git" ? path.dirname(commonDir) : "";
}

function matchProjectPattern(pattern, project) {
  const normalizedPattern = normalizeMaybePathPattern(pattern);
  return projectIdentities(project).some((identity) => wildcardMatch(normalizedPattern, identity));
}

function matchProjectPath(pattern, project) {
  const normalizedPattern = normalizeMaybePathPattern(pattern);
  return [project.currentPath, project.commonPath].some((identity) => wildcardMatch(normalizedPattern, identity));
}

function projectIdentities(project) {
  return [
    project.currentName,
    project.commonName,
    project.currentPath,
    project.commonPath
  ].filter(Boolean);
}

function normalizeMaybePathPattern(pattern) {
  const expanded = expandHome(pattern);
  if (!looksLikePath(expanded)) {
    return expanded;
  }
  if (expanded.includes("*")) {
    return path.resolve(expanded);
  }
  return normalizePath(expanded);
}

function looksLikePath(value) {
  return value === "~" || value.startsWith("~/") || value.startsWith("/") || value.startsWith(".");
}

function wildcardMatch(pattern, value) {
  if (pattern === value) {
    return true;
  }
  if (!pattern.includes("*")) {
    return false;
  }
  const escaped = pattern.replace(/[|\\{}()[\]^$+?.]/g, "\\$&").replace(/\*/g, ".*");
  return new RegExp(`^${escaped}$`).test(value);
}

function normalizePath(value) {
  const expanded = expandHome(value);
  const resolved = path.resolve(expanded);
  try {
    return fs.realpathSync.native(resolved);
  } catch {
    return resolved;
  }
}
