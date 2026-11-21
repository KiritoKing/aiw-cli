---
name: aiw-reference
description: Use the Go AIW CLI to select, materialize, inspect, repair, and release business Git worktrees inside an externally created Agent Repo Change worktree.
---

<!-- AIW-GO-WORKTREE-V1 -->

# AIW Reference

The control plane owns the Agent Repo worktree and Agent runtime. AIW owns only the business worktrees it creates and their `repos/` links. Use a working `git` and `aiw` executable; from an AIW source checkout, run `go run ./cmd/aiw`.

## Change flow

- During Propose or an equivalent planning stage, edit `.aiw/change.yaml` in the control-plane-created Agent Repo worktree. It contains `version: 1` and a `repos` list whose names occur in the root `aiw.yaml`. OpenSpec and other tools can produce this same neutral file; AIW does not parse their own spec files.
- Before dispatching business tasks, run `aiw change materialize`. It links all selected business worktrees, then runs each selected repo’s setup script from the current worktree’s `aiw.yaml`. Uncommitted changes to the manifest take effect immediately. Dispatch only after it succeeds or `aiw change status --json` reports root `ready: true`. AIW does not fetch, push, merge, or run a coding Agent.
- If the repository scope changes, update `.aiw/change.yaml` and rerun `materialize`; every run repeats setup for all selected repos. `aiw change add-repo <name>` and `remove-repo <name>` are optional helpers. Removing a repo runs its cleanup before releasing its clean worktree. A TTY can pick repos through `aiw change select`; for scripts, repeat `--repo <name>`.
- On setup failure, keep the mounted worktree and use `aiw change setup` for pending/failed repos or `aiw change setup <name>` to rerun one. If a repo removed from the selection has local files, inspect them and explicitly run `aiw change cleanup <name>` before retrying materialization. `aiw change repair` restores worktrees and links without replaying scripts; a previously running script has unknown effects and needs explicit retry. `aiw change list --json` reports local Changes in branch-name order and flags missing roots as `rootExists: false`. `aiw change path <branch>` returns an available root. `aiw doctor --json` diagnoses local prerequisites, project configuration, stores, and orphaned worktrees.
- Use `aiw repo list --json` for available repos and local registration; `aiw change status --json` reports each business repo's creation `baseSha`, HEAD, branch, path, and readiness. The root Git status does not include changes inside `repos/<name>`. Inspect diff and delivery state in each business repo (`git -C repos/<name> status` / `diff`), then use that repo's own tools and rules for tests, commits, pushes, and merge requests. Record the resulting SHAs and review links in the root Change context.

## VS Code workspace

`init` and the `materialize` / `repair` reconciliation flow generate `aiw.code-workspace` by default. It lists the Agent Repo and every repo in the current `aiw.yaml` using relative `repos/<name>` paths, including repos not selected or materialized. Set `vscode.autoSync: false` in `aiw.yaml` to disable automatic updates. Run `aiw workspace sync` explicitly from the Agent Repo root or a subdirectory to replace the entire file even when automatic updates are disabled; custom settings/tasks in that file are overwritten. The generated file is locally Git-ignored. Open it in VS Code yourself; generation does not create business worktrees or start an editor.

## Close boundary

The control plane's cleanup script calls `aiw change close`, then removes the root worktree only if close succeeds. AIW checks remote same-name branches and local changes before release, runs each repo cleanup, and checks discarded content again afterward. If a mounted repo is deleted from `aiw.yaml`, release uses its recorded store and cleanup definition, with historical config fallback for pre-existing state. Update `.aiw/change.yaml` before reconciling a smaller selection. A failed cleanup leaves the business worktree in place; use `aiw change cleanup <name>` for explicit retry. In a noninteractive workflow, use `--allow-unpushed` and `--discard-changes` only when that specific loss or missing push has been authorized.

Close removes AIW business worktrees and links, retains all branches, and leaves the Agent Repo worktree for its control plane. It does not push, merge, create PRs, or release the Agent Repo worktree.
