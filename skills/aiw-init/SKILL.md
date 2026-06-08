---
name: aiw-init
description: Initialize, bootstrap, configure, or troubleshoot the AIW CLI environment for a user. Use when the user asks about installing AIW, running `npx @chlrc/aiw init`, setting up cmux actions, choosing AIW config paths, checking dependency gates, diagnosing setup failures, or answering questions about the AIW initialization flow.
---

# AIW Init

## Purpose

Help users get AIW ready on a machine without turning setup into a blind write operation. AIW is a thin orchestration CLI for cmux, Worktrunk, lazygit, delta, yazi, nvim, and agent CLIs; setup should verify those boundaries and only write personal config when that is intended.

## Operating Rules

- Prefer a read-only preflight first. Use `doctor`, `init --dry-run`, or direct `command -v` checks before applying setup changes.
- Treat `aiw init` as a write operation: it can create `~/.config/aiw`, create code/worktree directories, and merge cmux config. Run it for real only when the user explicitly wants initialization to proceed.
- Do not use `--force` unless the user explicitly wants existing AIW config files overwritten. `--force` creates backups, but it is still a replacement operation.
- Do not claim that `aiw init` installs agent skills. Current AIW init prints `[skip] skills initialization`; install skills separately with the npm `skills` CLI.
- Keep AIW personal workflow config out of business repositories by default. Prefer `--cmux-scope home` unless the user specifically wants project-local cmux registration.

## Resolve the AIW Command

Use the package path first. Users do not need to clone the AIW repository just to initialize a machine:

```bash
npx @chlrc/aiw init --help
npx @chlrc/aiw init --dry-run --yes
```

After initialization, prefer the installed `aiw` binary for daily checks:

```bash
aiw --help
aiw doctor
```

For frequent use, local customization, or AIW development, a local checkout is recommended later:

```bash
node bin/aiw --help
```

When initializing from the scoped package, store the same launcher in cmux actions unless the user has already installed an `aiw` binary:

```bash
npx @chlrc/aiw init --launcher "npx @chlrc/aiw" --dry-run --yes
```

The current CLI default launcher is `npx aiw`; `--launcher` controls the command prefix written into cmux actions such as `aiw-new-worktree`.

## Setup Workflow

1. Identify the user's target paths and cmux scope:
   - Config: `~/.config/aiw`, or `$AIW_CONFIG_DIR` when set.
   - Code root: default `~/Code`.
   - Worktrees root: default `~/worktrees`.
   - cmux scope: `home`, `code`, or `none`.

2. Run dependency and setup preflight:

```bash
aiw doctor
aiw doctor --gate init --agent codex
aiw init --dry-run --yes
```

For first-time package bootstrap:

```bash
npx @chlrc/aiw init --launcher "npx @chlrc/aiw" --dry-run --yes
```

3. Explain blockers using the preflight output. Blocking setup dependencies include Node/npx, Git, Worktrunk (`wt`), cmux, yazi, lazygit, nvim, the default layout/commit agent, and `delta` when the lazygit overlay is configured. Optional tools such as `fd`, `eza`, and non-default agents should be reported without blocking unrelated setup.

4. Apply only after the user intends it:

```bash
npx @chlrc/aiw init --launcher "npx @chlrc/aiw" --yes
```

Useful variants:

```bash
npx @chlrc/aiw init --cmux-scope home --yes
npx @chlrc/aiw init --cmux-scope code --code-root ~/Code --worktrees-root ~/worktrees --yes
npx @chlrc/aiw init --cmux-scope none --dry-run --yes
npx @chlrc/aiw init --config-dir ~/.config/aiw --no-reload --yes
```

5. Verify after setup:

```bash
aiw doctor
aiw doctor --gate cmux-new --agent codex
aiw layout --agent codex --dry-run
```

When validating cmux registration, check that the AIW actions exist rather than editing cmux config manually:

- `aiw-new-worktree` -> `aiw cmux-new`
- `aiw-pick-directory` -> `aiw cmux-new --pick-repo`
- `aiw-local-workspace` -> `aiw cmux-new --local`

## Troubleshooting

- If setup fails before writing files, install the missing blocking dependency and rerun the dry-run command.
- If cmux config parsing fails, inspect the target JSON/JSONC file and fix invalid syntax before rerunning init.
- If an agent command is missing, install that agent CLI or edit `~/.config/aiw/agents.toml` to point at an available command.
- If `fnm_multishells ... Operation not permitted` appears before command output, treat it as shell startup noise when the actual AIW command succeeds.
- If the user only wants setup advice, answer from the preflight evidence and do not apply changes.
