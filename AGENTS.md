# AGENTS.md

This file applies to the entire `aiw` workspace.

`AGENTS.md` is an agent-facing project guide. Keep it short, operational, and aligned with the actual code and docs in this repo.

## Project Context

`aiw` is a personal AI programming workflow CLI. It is intentionally a thin orchestration layer:

- `aiw` owns workflow decisions, config loading, dependency gates, prompts, and command routing.
- Worktrunk owns worktree lifecycle.
- cmux owns workspaces and panes.
- lazygit owns Git TUI operations.
- delta owns diff rendering.
- agent CLIs own model interaction.
- yazi, nvim, rg, fd, fzf, bat, and eza keep their native responsibilities.

Do not turn `aiw` into a terminal emulator, Git client, editor, diff viewer, daemon, task database, or agent manager.

## Read First

Before making non-trivial changes, read:

- `README.md` for command behavior.
- `docs/2026-05-29-design.md` for product boundaries.
- `HANDOFF.md` for current progress and known gaps.
- `docs/2026-06-01-workflow-handoff.md` for the latest detailed handoff.

## Repository Layout

- `bin/aiw`: executable entrypoint.
- `src/cli.mjs`: top-level command dispatch.
- `src/config.mjs`: config loading and agent resolution.
- `src/deps.mjs`: dependency gates and doctor output.
- `src/git.mjs`: Git repo, repo picker, and branch selection helpers.
- `src/layout.mjs`: cmux layout generation.
- `src/commit.mjs`: AI commit workflow.
- `src/agent.mjs`: agent invocation and output cleanup.
- `src/run.mjs`: process execution helpers.
- `config/aiw.toml`: default workflow config.
- `config/agents.toml`: agent command adapters.
- `config/commit-prompt.md`: base AI commit prompt.
- `config/lazygit-delta.yml`: lazygit overlay used by `aiw git`.
- `docs/`: design notes and handoff records.

## Development Commands

Use the checked-out CLI while developing:

```bash
node bin/aiw --help
node bin/aiw doctor
node bin/aiw doctor --gate git
node bin/aiw doctor --gate cmux-new --agent codex
node bin/aiw layout --agent codex --dry-run
node bin/aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --agent codex --dry-run
```

Run this after code changes:

```bash
npm run check
```

There is no full automated test suite yet. For risky CLI behavior, create a temporary Git repo under `/private/tmp` and verify the relevant command non-interactively where possible.

## Coding Rules

- Use Node.js ESM and keep compatibility with Node >= 18.
- Keep the CLI dependency-light; do not add npm dependencies unless the workflow clearly needs them.
- Prefer small functions and explicit exit codes over broad abstractions.
- Keep config parsing and command routing predictable.
- In a TTY, missing required CLI arguments should open a searchable TUI/picker by default; non-interactive calls should error or preserve the underlying tool default behavior.
- When adding a command that depends on an external tool, add or update a dependency gate in `src/deps.mjs`.
- When adding user-facing behavior, update `README.md` or `docs/` in the same change.
- Do not start development servers from this repo.
- Do not write personal workflow files into business repositories by default.
- Do not auto-run `git init` in non-Git directories.

## Product Boundaries

Preserve these constraints:

- `cmux-new` and `layout` must pass dependency gates before creating worktrees or opening workspaces.
- `aiw commit` expects staged changes and should not silently stage files for the user.
- Commit generation must read staged diff, support custom prompt injection, commit via `git commit -F -`, and retry hook failures according to config.
- `aiw git` should load the AIW lazygit overlay by default, but direct `lazygit` should remain untouched.
- The right-bottom diff pane currently uses `cmux-git-diff` when available, otherwise `git diff | delta`; do not describe it as cmux built-in diff unless that becomes true.

## cmux Safety

Be careful with cmux customization.

- Do not edit a user's existing `<code-root>/.cmux/cmux.json` unless the user explicitly asks for that exact change.
- A previous attempt to inject AIW into cmux config broke the user's existing cmux reload behavior and was rolled back.
- Future cmux integration should be conservative: inspect current config, generate a preview, back up, validate with `cmux config check`, then ask before applying if the change affects existing actions.

## lazygit and AI Commit

`aiw git` injects `config/lazygit-delta.yml` using lazygit's `--use-config-file`.

Current lazygit overlay behavior:

- Uses delta for diff display.
- Adds `Ctrl-A` as a global lazygit custom command.
- `Ctrl-A` prompts for optional extra commit instructions and runs `aiw commit --prompt ...`.

Do not replace lazygit's native commit key. AI commit should remain an additional path.

## Validation Checklist

For code changes:

```bash
npm run check
```

For Git/lazygit changes:

```bash
node bin/aiw doctor --gate git
ruby -ryaml -e 'YAML.load_file("config/lazygit-delta.yml")'
```

For AI commit changes:

```bash
node bin/aiw doctor --gate commit --agent codex
```

Also verify with a temporary Git repo and staged changes when changing `src/commit.mjs`.

For cmux/worktree changes:

```bash
node bin/aiw doctor --gate cmux-new --agent codex
node bin/aiw layout --agent codex --dry-run
node bin/aiw cmux-new --repo <repo> --branch <branch> --agent codex --dry-run
```

## Known Environment Notes

- Do not assume every checkout has a remote configured; inspect Git state before push or release work.
- `fd` may be missing locally, which affects `aiw pick`.
- `cmux-git-diff` may be missing locally; `aiw diff` should fall back to delta.
- `aider` may be missing; this only affects selecting the aider agent.
- Shell output may include `fnm_multishells ... Operation not permitted`; ignore it if the actual command succeeded.

## Documentation Practice

- Keep repo-facing docs in Simplified Chinese when documenting workflow decisions for the user.
- Keep command examples copy-pasteable.
- When changing workflow behavior, update `handoff.md` if the next agent would need to know it.
- This project uses a file-contract handoff model. At the end of every task, maintain `handoff.md`: keep only the current work activity context, remove stale context, and add any context introduced or changed by the current task.
- If a work log is worth preserving beyond the current activity, write it under `docs/` with the date in the filename as the index.
- Dated work logs under `docs/` are historical records and are read-only by default. Do not modify existing logs unless the user explicitly asks; create a new dated log when there is valuable new history to preserve.
