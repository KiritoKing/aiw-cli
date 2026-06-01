# aiw

`aiw` is a personal AI programming workflow CLI. It turns a pile of sharp terminal tools into one predictable loop for starting work, opening an AI-ready workspace, reviewing changes, generating commits, and cleaning up worktrees.

Default language: English. For the Chinese version, see [README.zh-CN.md](./README.zh-CN.md).

## What aiw Is

AIW is intentionally a thin orchestration layer. It does not replace the tools you already use. It decides what should happen next, checks that required dependencies exist, and then delegates the actual work to the right tool.

The boundary is:

- `aiw` owns workflow decisions, config loading, dependency gates, prompts, command routing, and cmux layout generation.
- Worktrunk owns worktree lifecycle.
- cmux owns workspaces and panes.
- lazygit owns Git TUI operations.
- delta owns diff rendering.
- agent CLIs own model interaction.
- yazi, nvim, rg, fd, fzf, bat, and eza keep their native responsibilities.

The result is a CLI that keeps a personal workflow consistent without turning itself into a terminal emulator, Git client, editor, diff viewer, daemon, task database, or agent manager.

## Quick Start

From this checkout:

```bash
node bin/aiw --help
node bin/aiw doctor
node bin/aiw cmux-new --agent codex
```

Bootstrap with the published package, then use the installed `aiw` binary:

```bash
npx @chlrc/aiw init
aiw doctor
aiw cmux-new --agent codex
```

The normal daily loop is:

```bash
# 1. Check whether the local toolchain is ready.
aiw doctor

# 2. Create or switch to a Worktrunk worktree and open the AIW cmux layout.
aiw cmux-new --agent codex

# 3. Work in the four-pane workspace.
# Files: yazi
# Agent: codex / claude / opencode / gemini / aider
# Git: lazygit with the AIW overlay
# Diff: cmux-git-diff or git diff piped through delta

# 4. Review and stage changes.
aiw git

# 5. Generate a commit message from staged changes and commit.
aiw commit

# 6. Merge and clean up a feature worktree when it is done.
aiw done dev
```

Run the checked-out CLI while developing AIW itself:

```bash
node bin/aiw doctor
node bin/aiw doctor --gate cmux-new --agent codex
node bin/aiw layout --agent codex --dry-run
node bin/aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --agent codex --dry-run
```

## Installation Model

AIW is a Node.js ESM CLI targeting Node.js 18 or newer. The executable entrypoint is [bin/aiw](./bin/aiw), and package metadata exposes it as the `aiw` binary.

For local development, calling `node bin/aiw ...` is enough. For a global command, use the project install script if available in your checkout:

```bash
npm run install:global
```

AIW reads configuration from:

1. `$AIW_CONFIG_DIR`, when set.
2. `~/.config/aiw`, when present.
3. The repository default [config/](./config) directory.

This lets the repo provide a usable default while still allowing a personal config directory outside business repositories.

## Init

Use `npx @chlrc/aiw init` on macOS or Linux to bootstrap a machine for AIW.

The init flow assumes Node.js and `npx` are already available. It checks required environment variables, the POSIX shell platform, and blocking tools for the default workflow. If any blocking dependency is missing, it stops before writing files and prints install guidance. Optional tools such as `fd`, `eza`, and non-default agent CLIs are reported without blocking.

When the blocking gate passes, init creates missing directories and config files:

```bash
npx @chlrc/aiw init
npx @chlrc/aiw init --cmux-scope home
npx @chlrc/aiw init --cmux-scope code --code-root ~/Code --worktrees-root ~/worktrees
npx @chlrc/aiw init --cmux-scope none --dry-run
```

- AIW config defaults to `~/.config/aiw`, or `$AIW_CONFIG_DIR` when set.
- `--code-root` and `--worktrees-root` control the paths written into `aiw.toml`; defaults are `~/Code` and `~/worktrees`.
- cmux registration defaults to `~/.config/cmux/cmux.json`; use `--cmux-scope code` to write `<code-root>/.cmux/cmux.json`, or `--cmux-scope none` to skip cmux.
- In an interactive terminal, init asks where to register cmux. `--yes` uses the default without prompting.
- Existing AIW config files are kept by default. Use `--force` to overwrite them after backups are created.
- Existing cmux config is merged, not replaced. AIW actions are added and existing non-AIW plus-button actions are preserved by putting AIW entries in the context menu.
- Skills are intentionally not initialized yet.

## Technology Stack

AIW is deliberately dependency-light:

- Runtime: Node.js ESM, Node >= 18.
- Config: small TOML files parsed by AIW itself.
- Process execution: Node child processes through local helpers.
- Workspace orchestration: Worktrunk (`wt`) and cmux.
- Git UI: lazygit, with an AIW-specific overlay loaded only by `aiw git`.
- Diff UI: `cmux-git-diff` when installed, otherwise `git diff | delta`.
- Pickers and search: fzf, rg, fd, bat.
- File and edit surfaces: yazi and nvim.
- Agents: command adapters defined in [config/agents.toml](./config/agents.toml).

The project currently has no full automated test suite. The required check after code changes is:

```bash
npm run check
```

For risky CLI behavior, create a temporary Git repo under `/private/tmp` and verify the relevant command non-interactively where possible.

## Mental Model

Think of AIW in three layers:

1. cmux is the front door. The plus button, command palette, or a cmux terminal action is where new work should begin.
2. `aiw` is the orchestration entrypoint. It resolves the repo, branch/worktree, agent, dependency gates, and layout, then calls the right lower-level command.
3. Mature CLI tools do the real work: Worktrunk for worktrees, yazi for files, nvim/LazyVim for editing, lazygit for review and staging, delta for diffs, rg/fd/fzf/bat/eza for search and inspection, zoxide in the surrounding shell workflow, and agent CLIs for model interaction.

AIW started from a practical goal: recreate the useful Codex App-style workspace motion without moving the workflow into a closed app. The target experience is not just "four panes on screen." It is a continuous path: open cmux, start AIW, create or switch a worktree, land in the standard workspace, inspect files, talk to an agent, stage hunks, watch the diff, commit, merge, and clean up.

The important difference is control. A polished app can be pleasant, but it tends to fix the shape of the workflow. In a terminal stack, you can choose the file manager, editor setup, Git UI, diff renderer, fuzzy finder, navigation tools, shell behavior, keybindings, and agent CLI. Shell rendering also matters: when command output is central to development, native terminal tools are easier to trust, tune, and compose than an embedded shell surface.

AIW therefore keeps the Codex-style workspace pattern, but makes it cmux-first, terminal-native, and agent-neutral. The same motion should run with Codex, Claude, opencode, Gemini, aider, or any future agent adapter that follows the same config contract. If you care about deep customization and want development control back in your own terminal stack, AIW is the experiment.

When you start work, AIW should answer four questions:

1. Which repository am I working in?
2. Which worktree or branch should hold this work?
3. Which agent should be opened next to the code?
4. Which standard panes should be visible while I work?

Once those are known, AIW stops making domain decisions and delegates:

```text
aiw cmux-new
  -> dependency gate
  -> repository and branch selection
  -> Worktrunk creates or switches the worktree
  -> aiw layout builds the cmux workspace
  -> cmux opens Files / Agent / Git / Diff panes
```

The same idea applies to cleanup:

```text
aiw workspace gc
  -> read Worktrunk and Git state
  -> mark dirty, merged, stale, and cmux-open signals
  -> remove only clean + merged worktrees when explicitly confirmed
```

AIW is not meant to hide the underlying tools. It is meant to make the common path short and the dangerous path explicit.

## Design Logic

### cmux Is the Entry, AIW Is the Orchestrator

The intended entry is cmux, not a standalone AIW dashboard. cmux already owns windows, workspaces, panes, and the user's visible development surface. AIW sits behind that entry as the command that decides what to open and how to wire it together.

That is why the main motion is:

```text
cmux entry
  -> aiw cmux-new
  -> dependency gate
  -> Worktrunk worktree create/switch
  -> aiw layout
  -> cmux workspace with Files / Agent / Git / Diff
```

The value is the fluid handoff between layers. cmux stays the place you start and look. AIW stays the workflow brain. The lower-level CLIs stay sharp and native.

### Recreate the App Motion, Not the App Lock-In

AIW borrows the strongest idea from a Codex App-style interface: keep the code workspace, agent, Git review, and diff loop together, and make starting isolated work feel natural. It does not borrow the lock-in. The workflow is expressed as commands, config files, and a cmux layout, so each layer remains replaceable.

That is why agent support is adapter-based. An agent is not hardcoded into the product. It is a command plus optional args for an interactive pane, and separate `commit_args` for one-shot commit generation.

### Keep Personal Workflow Out of Business Repos

AIW should not write `.aiw`, `.worktrunk`, or `.cmux` files into business repositories by default. Runtime metadata that AIW owns, such as recorded `feature -> target` workspace targets, is stored under Git's common dir rather than in the working tree.

### Let the Terminal Stay the Control Plane

AIW assumes the user should be able to swap tools, tune rendering, keep native shell behavior, and use existing muscle memory. It composes mature tools such as rg, zoxide, yazi, nvim/LazyVim, lazygit, delta, fzf, Worktrunk, cmux, and agent CLIs instead of flattening them into a new closed UI.

This is also why shell and diff output stay in terminal-native tools. The terminal is not a fallback surface for AIW; it is the primary control plane.

### Gate Before Side Effects

Commands that create worktrees or open cmux layouts run dependency gates first. If an external tool is missing, AIW stops before making a new workspace.

Useful examples:

```bash
aiw doctor
aiw doctor --gate git
aiw doctor --gate cmux-new --agent codex
aiw doctor --gate commit --agent codex
aiw doctor --json
```

### Interactive When Human, Explicit When Scripted

In a TTY, missing required arguments open searchable pickers where that makes sense. In non-interactive calls, pass explicit arguments or rely on the underlying tool's own behavior.

Examples:

```bash
aiw cmux-new
aiw cmux-new --pick-repo
aiw workspace open
aiw workspace open feat/foo --agent codex
```

### Preserve Native Tool Responsibility

`aiw git` opens lazygit. It does not reimplement staging, hunk selection, or commit review.

`aiw files` opens yazi. It does not become a file manager.

`aiw diff` renders the current Git diff. It does not become a custom diff engine.

## Core Commands

```bash
aiw doctor
aiw cmux-new --agent codex
aiw cmux-new --pick-repo --agent codex
aiw cmux-new --local --agent codex
aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --agent codex --dry-run
aiw layout --agent codex --dry-run

aiw workspace list
aiw workspace list --json
aiw workspace open
aiw workspace open feat/foo --agent codex
aiw workspace done dev
aiw workspace remove feat/foo
aiw workspace gc
aiw workspace gc --dry-run
aiw workspace gc --apply

aiw git
aiw diff
aiw diff --watch
aiw diff --staged
aiw commit
aiw commit --agent codex
aiw commit --prompt "Use scope aiw"
aiw commit --prompt-file ~/commit-style.md

aiw files
aiw edit src/file.ts:10
aiw grep keyword
aiw pick
aiw tree 3
```

Short workspace aliases are kept for daily use:

```bash
aiw ws list
aiw list
aiw open feat/foo
aiw switch feat/foo
aiw done
aiw remove feat/foo
aiw gc
aiw clean
```

## Worktree and cmux Workflow

`aiw cmux-new` is the main entrypoint for new work.

Common forms:

```bash
aiw cmux-new
aiw cmux-new --agent codex
aiw cmux-new --pick-repo --agent codex
aiw cmux-new --repo ~/Code/my-repo --agent codex
aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --agent codex
aiw cmux-new --local --agent codex
```

Behavior:

- The current Git repository is used automatically when possible.
- `--pick-repo` lets you select another repository under `paths.code_root`.
- Without a branch in a TTY, AIW lets you create a new branch from current `HEAD`, open the current checkout, or choose an existing branch.
- Creating a new branch runs `wt switch --create <branch> --base @ -x "aiw layout --agent <agent>"`.
- Selecting an existing branch runs `wt switch <branch> -x "aiw layout --agent <agent>"`.
- `--local` opens the standard layout in the current checkout without creating a Worktrunk worktree.
- `--dry-run` prints the command that would be run.

The standard cmux layout is:

```text
+----------------------+----------------------+
| Files                | Agent                |
| aiw files            | codex/claude/etc.    |
+----------------------+----------------------+
| Git                  | Diff                 |
| aiw git              | aiw diff --watch     |
+----------------------+----------------------+
```

`aiw layout --print-json` prints the cmux layout JSON without opening cmux.

## Workspace Management

`aiw workspace ...` is the visibility and lifecycle layer around Worktrunk worktrees.

### List

```bash
aiw workspace list
aiw workspace list --json
aiw workspace list --stale-seconds 604800
aiw workspace states
```

The table combines Worktrunk, Git, and cmux signals:

- branch and path
- dirty or clean state
- Worktrunk integration state
- recorded merge target, if AIW created the worktree
- inferred merged target, when no recorded target exists
- cmux-open status
- age
- GC signal

State markers are intentionally conservative. Dirty worktrees are never automatic cleanup candidates.

### Open

```bash
aiw workspace open
aiw workspace open feat/foo --agent codex
aiw workspace open /path/to/worktree --agent codex
aiw open feat/foo
```

In a TTY, `workspace open` without a target opens a searchable picker over existing worktrees and local branches. A branch target is opened through Worktrunk. A path target opens the AIW cmux layout directly in that Git root.

### Done

```bash
aiw workspace done
aiw workspace done dev
aiw workspace done dev --no-close-cmux
aiw done dev
```

`done` is only allowed from a feature worktree. It refuses to run from the main workspace. It also refuses to proceed when the current worktree is dirty.

When it proceeds, AIW delegates the merge and cleanup to Worktrunk:

```bash
wt merge <target>
```

If AIW recorded a target when the worktree was created, bare `aiw done` can use that target. In a TTY it opens a target picker when needed. After a successful merge, AIW closes the matching cmux workspace unless `--no-close-cmux` is passed.

### Remove and GC

```bash
aiw workspace remove feat/foo
aiw workspace remove feat/foo --force
aiw workspace gc
aiw workspace gc --dry-run
aiw workspace gc --apply
aiw workspace gc --yes
aiw workspace gc --json
```

`remove` performs a dirty check before calling `wt remove`. Use `--force` only when you intend to hand that decision to Worktrunk.

`gc` separates three signals:

- `dirty`: blocks automatic cleanup.
- `merged`: the branch is integrated, same commit, empty, or known to be contained by a target.
- `stale`: last commit age is greater than `workspace.stale_seconds`, defaulting to seven days.

Only clean + merged worktrees are removable. Stale dirty or unmerged worktrees are warning-only.

### Workspace hooks

Workspace hooks can run shell commands before AIW initializes a cmux workspace or removes a worktree. Configure global hooks in `~/.config/aiw/aiw.toml` and project hooks in an explicit `.aiw.toml` at the Git repository root. AIW reads project hooks only when the file already exists; it does not create project-local workflow files.

```toml
[workspace.hooks]
pre_init = ["echo preparing $AIW_WORKSPACE_PATH"]
pre_remove = ["echo removing $AIW_WORKSPACE_TARGET"]

[workspace.hooks.projects.my-repo]
pre_init = ["echo preparing only $AIW_PROJECT_NAME"]
pre_remove = ["echo cleaning only $AIW_PROJECT_PATH"]

[workspace.hooks.projects.my-alias]
match = "repo.name.with.dots"
path = "~/Code/repo.name.with.dots"
pre_init = ["echo project matched by name or path"]
```

Global hooks run first, then matching global project hooks, then project-local hooks. A `[workspace.hooks.projects.<name>]` table matches `<name>` against the current worktree name and the Git common-dir project name; use `match`, `path`, `paths`, `repo`, or `repos` when the table name is only an alias or when you need path matching. `*` wildcards are supported.

Commands run sequentially with `sh -lc` from the target workspace path; a failing hook stops the init/remove action. `pre_init` runs before `aiw layout` calls `cmux new-workspace`. `pre_remove` runs before `aiw workspace done`, `aiw workspace remove`, and `aiw workspace gc --apply` delete paths through Worktrunk.

Hook commands receive `AIW_HOOK_EVENT`, `AIW_HOOK_SOURCE`, `AIW_HOOK_CONFIG`, `AIW_HOOK_RULE`, `AIW_REPO`, `AIW_PROJECT_NAME`, `AIW_PROJECT_PATH`, `AIW_WORKSPACE_PATH`, `AIW_WORKSPACE_BRANCH`, `AIW_WORKSPACE_TARGET`, and `AIW_AGENT`.

## Git, Diff, and AI Commit

### lazygit Overlay

`aiw git` opens lazygit with the AIW overlay:

```bash
lazygit --use-config-file <aiw-config-dir>/lazygit-delta.yml
```

The overlay:

- uses delta for diff display
- adds `Ctrl-A` as a global lazygit custom command
- prompts for optional AI commit instructions
- runs `aiw commit --prompt ...`

Directly running `lazygit` is untouched.

### Diff

```bash
aiw diff
aiw diff --watch
aiw diff --staged
aiw diff --all
```

For an unstaged diff, AIW uses `cmux-git-diff` when available. Otherwise it pipes Git diff output through delta. `--watch` refreshes every two seconds.

### AI Commit

`aiw commit` expects staged changes. It never silently stages files.

```bash
aiw git
aiw commit
aiw commit --agent codex
aiw commit --prompt "Use scope aiw"
aiw commit --prompt-file ~/commit-style.md
aiw commit --print-prompt
aiw commit --dry-run
```

Flow:

1. Read `git diff --cached`, `git diff --cached --stat`, and `git status --short`.
2. Build a prompt from [config/commit-prompt.md](./config/commit-prompt.md).
3. Append `--prompt` and `--prompt-file` content when provided.
4. Call the selected agent using `commit_args` from [config/agents.toml](./config/agents.toml).
5. Clean the agent output into a commit message.
6. Run `git commit -F -`.
7. If commit hooks fail, include the hook output in the next prompt and retry up to `commit.retries`.

The agent adapter contract is stdin/stdout oriented:

- stdin or `{{prompt}}`: full commit prompt and staged diff
- stdout: commit message only
- stderr: logs and diagnostics

## Configuration

Default config files live in [config/](./config):

- [config/aiw.toml](./config/aiw.toml): defaults, paths, behavior, commit, Git, and workspace settings
- [config/agents.toml](./config/agents.toml): agent command adapters
- [config/commit-prompt.md](./config/commit-prompt.md): base commit prompt
- [config/lazygit-delta.yml](./config/lazygit-delta.yml): lazygit overlay used only by `aiw git`

Important defaults:

```toml
[defaults]
agent = "codex"
editor = "nvim"
files = "yazi"
git = "lazygit"

[paths]
code_root = "~/Code"
worktrees = "~/worktrees"

[commit]
agent = "codex"
retries = 3
max_diff_chars = 120000

[workspace]
stale_seconds = 604800

[workspace.hooks]
pre_init = []
pre_remove = []
```

Agent entries look like this:

```toml
[agents.codex]
cmd = "codex"
args = []
commit_args = ["exec", "--skip-git-repo-check", "--sandbox", "read-only", "--color", "never", "-"]
```

Use `args` for the interactive agent pane and `commit_args` for one-shot commit generation.

## Dependency Gates

`aiw doctor` reports tool availability and evaluates a gate. The default gate is `p0`.

Useful gates:

```bash
aiw doctor --gate git
aiw doctor --gate workspace
aiw doctor --gate layout --agent codex
aiw doctor --gate cmux-new --agent codex
aiw doctor --gate init --agent codex
aiw doctor --gate commit --agent codex
```

Gate behavior:

- `cmux-new` requires Git, Worktrunk, cmux, yazi, lazygit, nvim, the selected agent, and either `cmux-git-diff` or delta.
- `layout` requires the layout tools and selected agent, but not Worktrunk.
- `init` checks the tools needed to bootstrap the default workflow before writing config.
- `workspace` requires Git and Worktrunk.
- `git` requires lazygit and delta when the lazygit overlay is configured.
- `commit` requires Git and the selected commit agent.

Missing optional tools only matter when their command or gate needs them. For example, missing `fd` affects `aiw pick`, not `aiw commit`.

## Troubleshooting

### `dependency gate ... failed`

Run the matching doctor command:

```bash
aiw doctor --gate cmux-new --agent codex
```

Install the missing tool or change the AIW config to use a different command.

### `no staged changes`

`aiw commit` only reads staged changes. Stage files in lazygit or with Git first:

```bash
aiw git
git add <path>
aiw commit
```

### `not inside a Git repository`

Most AIW workflows are Git-rooted. Run the command from a Git repository, pass `--repo`, or use `--pick-repo` where supported.

### `aiw workspace done must be run from a feature worktree`

`done` is a feature worktree cleanup command. Run it inside the feature worktree, not the main checkout.

### Dirty Worktree Blocks Remove, Done, or GC

AIW protects uncommitted changes. Review them first:

```bash
aiw git
```

Use `--force` only on `workspace remove` when that is the explicit intention.

### Shell Prints `fnm_multishells ... Operation not permitted`

This can appear in the local environment before command output. If the actual AIW command succeeds, treat it as shell startup noise rather than an AIW failure.

## Development Guide

Read these before non-trivial changes:

- [README.md](./README.md): human-facing usage manual
- [AGENTS.md](./AGENTS.md): agent-facing development rules
- [docs/2026-05-29-design.md](./docs/2026-05-29-design.md): original design note
- [docs/2026-06-01-workflow-handoff.md](./docs/2026-06-01-workflow-handoff.md): workflow history
- [docs/2026-06-01-workspace-management-plan.md](./docs/2026-06-01-workspace-management-plan.md): workspace management record
- [handoff.md](./handoff.md): current working context

Run this after code changes:

```bash
npm run check
```

Additional targeted checks:

```bash
node bin/aiw doctor --gate git
ruby -ryaml -e 'YAML.load_file("config/lazygit-delta.yml")'
node bin/aiw doctor --gate commit --agent codex
node bin/aiw doctor --gate cmux-new --agent codex
node bin/aiw layout --agent codex --dry-run
node bin/aiw init --cmux-scope none --dry-run --no-reload
```

When changing `src/commit.mjs`, verify against a temporary Git repo with staged changes. When changing worktree behavior, prefer dry-run commands first and then a disposable repo under `/private/tmp`.

## Project Layout

```text
bin/aiw                    executable entrypoint
src/cli.mjs                top-level command dispatch
src/config.mjs             config loading and agent resolution
src/deps.mjs               dependency gates and doctor output
src/git.mjs                Git repo, repo picker, branch selection helpers
src/layout.mjs             cmux layout generation
src/hooks.mjs              workspace hook loading and execution
src/init.mjs               first-time machine bootstrap
src/workspace.mjs          workspace list/open/done/remove/gc
src/commit.mjs             AI commit workflow
src/agent.mjs              agent invocation and output cleanup
src/prompt.mjs             TTY input and fzf picker helpers
src/run.mjs                process execution helpers
config/aiw.toml            default workflow config
config/agents.toml         agent adapters
config/commit-prompt.md    base AI commit prompt
config/lazygit-delta.yml   lazygit overlay used by aiw git
docs/                      design notes and dated handoff records
handoff.md                 current activity handoff
```

## Status

AIW currently focuses on four stable loops:

- open or create worktrees with an AI-ready cmux layout
- inspect and clean up workspaces safely
- review changes through lazygit and delta
- generate commit messages from staged diffs through agent CLIs

Future work should preserve the same boundary: AIW coordinates; specialized tools do the specialized work.
