# aiw

`aiw` is a personal AI programming workflow CLI. It keeps the daily loop predictable: start isolated work, open an AI-ready workstation, review changes, generate commits, and clean up worktrees.

Default language: English. For the Chinese version, see [README.zh-CN.md](./README.zh-CN.md).

## What aiw Is

AIW is intentionally a thin orchestration layer. It decides the workflow path, checks dependencies, loads config, builds prompts, and delegates the actual UI and development work to mature tools.

- `aiw` owns workflow decisions, config loading, dependency gates, prompts, command routing, and workstation layout plans.
- Worktrunk owns worktree lifecycle.
- tmux owns the default workstation panes in any terminal, including Ghostty and remote/headless shells.
- cmux is an optional runtime integration: when AIW runs inside cmux, it opens cmux workspaces instead of tmux sessions.
- lazygit owns Git TUI operations.
- delta owns diff rendering.
- agent CLIs own model interaction.
- yazi, nvim, rg, fd, fzf, bat, and eza keep their native responsibilities.

AIW should not become a terminal emulator, Git client, editor, diff viewer, daemon, task database, or agent manager.

## Quick Start

Bootstrap with the published package:

```bash
npx @chlrc/aiw init
aiw doctor
aiw new --agent codex
```

From a local checkout, mostly for AIW development:

```bash
node bin/aiw --help
node bin/aiw doctor
node bin/aiw new --agent codex --dry-run
```

The normal daily loop is:

```bash
# 1. Check whether the local toolchain is ready.
aiw doctor

# 2. Create or switch to a Worktrunk worktree and open the runtime workstation.
aiw new --agent codex

# 3. Work in the standard panes.
# Project: Files + Agent on top, Git on bottom.
# Scratch: Files + Agent.

# 4. Review and stage changes.
aiw git

# 5. Generate a commit message from staged changes and commit.
aiw commit

# 6. Merge and clean up a feature worktree when it is done.
aiw done dev --no-close-ui
```

Compatibility aliases remain available:

```bash
aiw cmux-new --agent codex
aiw cmux scratch --agent codex
aiw done dev --no-close-cmux
```

Use `aiw new`, `aiw scratch`, and `--no-close-ui` in new docs, scripts, and generated config.

## Workstation Runtime

AIW supports two UI runtimes:

- `tmux`: required. AIW uses it by default in ordinary terminals, Ghostty, SSH, and headless development machines.
- `cmux`: recommended but optional. AIW uses it only when the current process is running inside cmux and the `cmux` CLI is available.

There is no user-facing backend choice. Runtime detection keeps the same command working across local terminals, Ghostty, tmux, SSH hosts, and cmux:

```bash
aiw layout --agent codex --dry-run
aiw scratch --agent codex --dry-run
```

Dependency gates treat `tmux` as blocking and `cmux` as recommended:

```bash
aiw doctor --gate new --agent codex
aiw doctor --gate layout --agent codex
aiw doctor --gate scratch --agent codex
```

If `cmux` is missing, `doctor` reports it as a recommended missing dependency. `aiw init` requires `--yes` or an interactive confirmation before continuing without cmux.

## Config and Migration

AIW reads configuration from:

1. `$AIW_CONFIG_DIR`, when set.
2. `~/.config/aiw`, when present.
3. The repository default [config/](./config) directory.

Older versions may have written a `[workstation]` section. That selection is now legacy because runtime detection chooses tmux or cmux automatically:

```bash
aiw migrate --dry-run
aiw migrate --dry-run --json
aiw migrate --yes
```

`aiw migrate` backs up `aiw.toml` before writing and removes the legacy `[workstation]` section. Other old fields are kept so a previous AIW version can still roll back.

## Init

`aiw init` writes personal config and can optionally register cmux actions:

```bash
npx @chlrc/aiw init --dry-run --yes
npx @chlrc/aiw init --cmux-scope home --yes
npx @chlrc/aiw init --cmux-scope none --yes
```

`tmux` is required. `cmux` is recommended: when it is unavailable, a real init run requires `--yes` or an interactive confirmation to continue with tmux-only setup.

Existing AIW config files are kept by default. Use `--force` only when replacing them after backups are created.

## Commands

Common workstation and lifecycle commands:

```bash
aiw doctor
aiw doctor --gate new --agent codex
aiw new --repo ~/Code/my-repo --branch feat/foo --agent codex --dry-run
aiw layout --agent codex --dry-run
aiw scratch --agent codex --root /private/tmp/aiw-sessions --id smoke --dry-run
aiw workspace list
aiw workspace open feat/foo --agent codex
aiw workspace gc --dry-run
aiw workspace done dev --agent codex --no-close-ui
```

Git and local surfaces:

```bash
aiw git
aiw diff
aiw diff --watch
aiw files
aiw edit src/file.ts:10
aiw grep keyword
aiw tree 3
```

`aiw git` opens lazygit with the AIW overlay. The overlay adds `Ctrl-A` for staged-change AI commits while leaving native lazygit commit behavior available.

## Development

Run this after code changes:

```bash
npm run check
```

For workstation/worktree changes:

```bash
node bin/aiw doctor --gate new --agent codex
node bin/aiw doctor --gate layout --agent codex
node bin/aiw layout --agent codex --dry-run
node bin/aiw new --repo /private/tmp/repo --branch feat/foo --agent codex --dry-run
```

There is no full automated test suite yet. For risky CLI behavior, create a temporary Git repo under `/private/tmp` and verify non-interactively where possible.

## Agent Skills

AIW ships skills in the npm `skills` CLI-compatible multi-skill layout:

- [skills/aiw-init](./skills/aiw-init/SKILL.md): bootstrap, migrate, and troubleshoot AIW setup.
- [skills/aiw-reference](./skills/aiw-reference/SKILL.md): operate AIW workstation, Git, commit, done, remove, and GC workflows.

Install from this repository:

```bash
npx --yes skills add KiritoKing/aiw-cli --list -y
npx --yes skills add KiritoKing/aiw-cli --skill aiw-init -y
npx --yes skills add KiritoKing/aiw-cli --skill aiw-reference -y
```

`aiw init` does not install these skills; it only initializes AIW config and optional cmux integration.
