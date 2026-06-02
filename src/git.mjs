import fs from "node:fs";
import path from "node:path";
import { capture, tryCapture } from "./run.mjs";
import { pickFromList } from "./prompt.mjs";

const SKIP_DIRS = new Set([
  ".git",
  "node_modules",
  ".pnpm-store",
  ".turbo",
  ".next",
  "dist",
  "build",
  "coverage"
]);

export function gitRoot(cwd = process.cwd()) {
  const result = tryCapture("git", ["rev-parse", "--show-toplevel"], { cwd });
  return result.ok ? result.stdout : "";
}

export function assertGitRoot(cwd = process.cwd()) {
  const root = gitRoot(cwd);
  if (root) {
    return root;
  }
  const error = new Error(`not inside a Git repository: ${cwd}`);
  error.exitCode = 3;
  throw error;
}

export function isDirty(repo) {
  const result = tryCapture("git", ["status", "--porcelain"], { cwd: repo });
  return result.ok && result.stdout.length > 0;
}

export async function resolveRepo(cwd, codeRoot, explicitRepo, options = {}) {
  if (explicitRepo) {
    const root = gitRoot(explicitRepo);
    if (!root) {
      throw withExit(`not a Git repository: ${explicitRepo}`, 3);
    }
    return root;
  }

  if (options.pickRepo) {
    return pickRepoFromRoot(codeRoot);
  }

  const current = gitRoot(cwd);
  if (current) {
    return current;
  }

  const normalizedCodeRoot = path.resolve(codeRoot);
  const normalizedCwd = path.resolve(cwd);
  if (!isInside(normalizedCwd, normalizedCodeRoot)) {
    throw withExit(`not inside a Git repository and not under code root: ${cwd}`, 3);
  }

  const repos = discoverRepos(normalizedCwd === normalizedCodeRoot ? normalizedCodeRoot : normalizedCwd, 3);
  if (repos.length === 0 && normalizedCwd !== normalizedCodeRoot) {
    repos.push(...discoverRepos(normalizedCodeRoot, 2));
  }
  if (repos.length === 0) {
    throw withExit(`no Git repositories found under ${normalizedCwd}`, 3);
  }
  return pickFromList("Select repository", repos);
}

export async function pickRepoFromRoot(codeRoot, defaultRepo = "") {
  const normalizedCodeRoot = path.resolve(codeRoot);
  const repos = discoverRepos(normalizedCodeRoot, 3);
  if (defaultRepo && !repos.includes(defaultRepo)) {
    repos.unshift(defaultRepo);
  }
  if (repos.length === 0) {
    throw withExit(`no Git repositories found under ${normalizedCodeRoot}`, 3);
  }
  return pickFromList("Select repository", repos, { defaultItem: defaultRepo });
}

export function currentBranch(repo) {
  const result = tryCapture("git", ["branch", "--show-current"], { cwd: repo });
  return result.ok ? result.stdout : "";
}

export function branchExists(repo, branch) {
  return Boolean(resolveBranchRef(repo, branch));
}

export function listBranches(repo) {
  const local = listRefs(repo, "refs/heads");
  const remoteRaw = listRefs(repo, "refs/remotes").filter((name) => !name.endsWith("/HEAD"));
  const remote = remoteRaw.map((name) => {
    if (name.startsWith("origin/")) {
      return name.slice("origin/".length);
    }
    return name;
  });
  return [...new Set([...local, ...remote])].sort();
}

export async function selectBranch(repo, requestedBranch, options = {}) {
  const current = currentBranch(repo);
  const requestedBase = options.baseBranch || "";
  const baseBranch = requestedBase ? resolveBranchRef(repo, requestedBase) : "";
  if (requestedBase && !baseBranch) {
    throw withExit(`base branch not found: ${requestedBase}`, 4);
  }
  if (requestedBranch) {
    const branchAlreadyExists = branchExists(repo, requestedBranch);
    const create = options.forceCreate || !branchAlreadyExists;
    if (requestedBase && !create) {
      throw withExit(`--base can only be used when creating a new branch: ${requestedBranch} already exists`, 2);
    }
    return {
      branch: requestedBranch,
      create,
      baseBranch: create ? baseBranch || "@" : "",
      targetBranch: create ? requestedBase || current : ""
    };
  }

  const branches = listBranches(repo).filter((branch) => branch !== current);
  if (requestedBase) {
    return createBranchSelection(repo, baseBranch, requestedBase);
  }

  const createChoice = "Create new branch from current HEAD...";
  const createFromBranchChoice = "Create new branch from existing branch...";
  const localChoice = "Open current checkout...";
  const selected = await pickFromList("Select worktree", [createChoice, createFromBranchChoice, localChoice, ...branches], {
    defaultItem: createChoice
  });
  if (selected === localChoice) {
    return {
      local: true
    };
  }
  if (selected !== createChoice) {
    if (selected === createFromBranchChoice) {
      const base = await pickFromList("Base branch", listBranches(repo), {
        defaultItem: current
      });
      return createBranchSelection(repo, resolveBranchRef(repo, base), base);
    }
    return {
      branch: selected,
      create: false
    };
  }

  return {
    ...(await promptNewBranch()),
    create: true,
    baseBranch: "@",
    targetBranch: current
  };
}

async function createBranchSelection(repo, baseBranch, targetBranch) {
  if (!baseBranch) {
    throw withExit(`base branch not found: ${targetBranch}`, 4);
  }
  return {
    ...(await promptNewBranch()),
    create: true,
    baseBranch,
    targetBranch
  };
}

async function promptNewBranch() {
  const { askInput } = await import("./prompt.mjs");
  const branch = await askInput("New branch");
  if (!branch) {
    throw withExit("branch is required", 4);
  }
  return {
    branch
  };
}

export function discoverRepos(root, maxDepth = 3) {
  const repos = [];
  walk(path.resolve(root), 0, maxDepth, repos);
  return [...new Set(repos)].sort();
}

function walk(dir, depth, maxDepth, repos) {
  if (depth > maxDepth) {
    return;
  }
  if (hasGitDir(dir)) {
    repos.push(dir);
    return;
  }
  let entries = [];
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return;
  }
  for (const entry of entries) {
    if (!entry.isDirectory() || SKIP_DIRS.has(entry.name)) {
      continue;
    }
    if (entry.name.startsWith(".") && entry.name !== ".config") {
      continue;
    }
    walk(path.join(dir, entry.name), depth + 1, maxDepth, repos);
  }
}

function hasGitDir(dir) {
  return fs.existsSync(path.join(dir, ".git"));
}

function listRefs(repo, refPath) {
  try {
    const output = capture("git", ["for-each-ref", "--format=%(refname:short)", refPath], { cwd: repo });
    return output ? output.split(/\r?\n/).filter(Boolean) : [];
  } catch {
    return [];
  }
}

function resolveBranchRef(repo, branch) {
  if (!branch) {
    return "";
  }
  const local = tryCapture("git", ["show-ref", "--verify", "--quiet", `refs/heads/${branch}`], { cwd: repo });
  if (local.ok) {
    return branch;
  }
  const remote = tryCapture("git", ["show-ref", "--verify", "--quiet", `refs/remotes/${branch}`], { cwd: repo });
  if (remote.ok) {
    return branch;
  }
  const originRemote = tryCapture("git", ["show-ref", "--verify", "--quiet", `refs/remotes/origin/${branch}`], { cwd: repo });
  return originRemote.ok ? `origin/${branch}` : "";
}

function isInside(child, parent) {
  const relative = path.relative(parent, child);
  return relative === "" || (!!relative && !relative.startsWith("..") && !path.isAbsolute(relative));
}

function withExit(message, exitCode) {
  const error = new Error(message);
  error.exitCode = exitCode;
  return error;
}
