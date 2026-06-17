---
name: aiw-init
description: Initialize, migrate, bootstrap, configure, or troubleshoot the AIW CLI environment for a user. Use when the user asks about installing AIW, running `npx @chlrc/aiw init`, migrating AIW config, setting up tmux/cmux workstation runtime support, choosing AIW config paths, checking dependency gates, diagnosing setup failures, or answering questions about the AIW initialization flow.
---

# AIW Init

## Purpose

Help users get AIW ready on a machine without turning setup into a blind write operation. AIW is a thin orchestration CLI for Worktrunk, tmux/cmux workstation runtime support, lazygit, delta, yazi, nvim, and agent CLIs.

## Operating Rules

- Prefer a read-only preflight first. Use `doctor`, `init --dry-run`, `migrate --dry-run`, or direct `command -v` checks before applying setup changes.
- Treat `aiw init` and `aiw migrate` as write operations. Run them for real only when the user explicitly wants initialization or migration to proceed.
- Do not use `--force` unless the user explicitly wants existing AIW config files overwritten.
- Do not claim that `aiw init` installs agent skills. Current AIW init prints `[skip] skills initialization`; install skills separately with the npm `skills` CLI.
- Keep AIW personal workflow config out of business repositories by default.
- Discuss cmux config registration as optional integration only; tmux remains the required runtime for normal use.

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

When initializing from the scoped package, store the scoped launcher in cmux actions unless the user has already installed an `aiw` binary:

```bash
npx @chlrc/aiw init --launcher "npx --yes @chlrc/aiw" --dry-run --yes
```

The current CLI default launcher is `npx --yes @chlrc/aiw`. Do not use `npx aiw`; that resolves to a different npm package.

## Workstation Runtime

AIW supports two runtime implementations:

- `tmux`: required. AIW uses it by default in normal terminals, Ghostty, SSH, and no-GUI machines.
- `cmux`: recommended but optional. AIW uses it only when running inside cmux and the `cmux` CLI is available.

Check current runtime and dependency gates:

```bash
aiw doctor --json
aiw doctor --gate new --agent codex
aiw doctor --gate layout --agent codex
aiw doctor --gate scratch --agent codex
```

Gate behavior:

- Missing `tmux` is blocking.
- Missing `cmux` is recommended/warn-level.
- A real `aiw init` run without cmux requires `--yes` or interactive confirmation to continue with tmux-only setup.

## Migration Workflow

Use `aiw migrate` to remove legacy `[workstation]` backend selection from an older config:

```bash
aiw migrate --dry-run
aiw migrate --dry-run --json
aiw migrate --yes
```

Rules:

- Migration backs up `aiw.toml` before writing.
- Existing legacy `[workstation]` is removed.
- No legacy `[workstation]` is a no-op by default.
- Other old fields are preserved by default for rollback.

## Setup Workflow

1. Identify the user's target paths:
   - Config: `~/.config/aiw`, or `$AIW_CONFIG_DIR` when set.
   - Code root: default `~/Code`.
   - Worktrees root: default `~/worktrees`.
   - Scratch sessions root: default `~/Documents/aiw`.
   - Optional cmux config scope: `home`, `code`, or `none`.

2. Run dependency and setup preflight:

```bash
aiw doctor
aiw doctor --gate init --agent codex
aiw init --dry-run --yes
```

For first-time package bootstrap:

```bash
npx @chlrc/aiw init --launcher "npx --yes @chlrc/aiw" --dry-run --yes
```

3. Explain blockers from the output:
   - Common blockers: Node/npx, Git, Worktrunk (`wt`), tmux, yazi, lazygit, nvim, the default layout/commit agent, and `delta` when the lazygit overlay is configured.
   - Recommended dependency: cmux.
   - Optional tools such as `fd`, `eza`, and non-default agents should be reported without blocking unrelated setup.

4. Apply only after the user intends it:

```bash
npx @chlrc/aiw init --launcher "npx --yes @chlrc/aiw" --yes
```

Useful variants:

```bash
npx @chlrc/aiw init --cmux-scope home --yes
npx @chlrc/aiw init --cmux-scope none --yes
npx @chlrc/aiw init --sessions-root ~/Documents/aiw --yes
npx @chlrc/aiw init --config-dir ~/.config/aiw --no-reload --yes
```

5. Verify after setup:

```bash
aiw doctor
aiw doctor --gate new --agent codex
aiw layout --agent codex --dry-run
aiw scratch --agent codex --dry-run
```

When validating cmux registration, check that the AIW actions exist rather than editing cmux config manually:

- `aiw-new-worktree` -> `aiw new`
- `aiw-pick-directory` -> `aiw new --pick-repo`
- `aiw-local-workspace` -> `aiw new --local`
- `aiw-scratch-session` -> `aiw scratch`
- `aiw-scratch-resume` -> `aiw scratch resume`

## Troubleshooting

- If setup fails before writing files, install the missing blocking dependency and rerun the dry-run command.
- If migration is a no-op, inspect whether legacy `[workstation]` exists.
- If cmux config parsing fails, inspect the target JSON/JSONC file and fix invalid syntax before rerunning init.
- If an agent command is missing, install that agent CLI or edit `~/.config/aiw/agents.toml` to point at an available command.
- If `fnm_multishells ... Operation not permitted` appears before command output, treat it as shell startup noise when the actual AIW command succeeds.
- If the user only wants setup advice, answer from the preflight evidence and do not apply changes.
