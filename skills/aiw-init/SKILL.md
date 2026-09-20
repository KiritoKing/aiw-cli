---
name: aiw-init
description: Set up the Go-based AIW CLI, initialize an Agent Repo, and register existing local business repositories for multi-repo Git worktree assembly.
---

<!-- AIW-GO-WORKTREE-V1 -->

# AIW Init

AIW assembles business Git worktrees inside an Agent Repo worktree created by a separate control plane. It needs Go 1.22+ to build and a working Git CLI at runtime. It does not configure tmux, cmux, Worktrunk, Agent sessions, or OpenSpec.

## Setup

1. Check `aiw --help` or, from an AIW source checkout, `go run ./cmd/aiw --help`. Run `aiw doctor` to verify Git and the current Agent Repo when applicable.
2. Run `aiw init <path>` only when the user asks to create or initialize that Agent Repo. The command may run `git init` in a new empty path; it refuses a nonempty non-Git directory. It writes `aiw.yaml`, a `/repos/` ignore rule, and an AIW-managed block in `AGENTS.md`, without committing them. It also invokes the community `npx skills add` CLI to install AIW skills from the public AIW repository at project scope. If Node or npx is absent, the Go binary installs its bundled snapshot without overwriting conflicting files. Use `--skip-skills` when the user will install skills with another tool.
3. Edit `aiw.yaml` with the stable repository set. Each repo needs a portable `remote` and a `base` branch; optional `setup` and `cleanup` entries use `command` and `args` and run inside the business worktree. Commit this config through the normal Git workflow; lifecycle commands read the committed Agent Repo baseline.
4. Use `aiw repo register <name> --source <existing-checkout>` or `aiw repo scan <directory>`. Scan stops descending when it finds `.git` and rejects ambiguous matches. Registration makes a machine-local bare store without editing the source checkout.
5. Use `aiw repo sync [name]` only when a fresh remote baseline is requested. Creating business worktrees never implicitly fetches or clones.

For an existing Agent Repo, run `aiw agents sync` to update only the text between `<!-- aiw:start -->` and `<!-- aiw:end -->`. Keep team rules outside that block. Use the community skills CLI or another installer for future skill updates; AIW has no skill updater. Until the public AIW repository publishes the Go-oriented skills, do not treat a successful remote skill install as validation of the new workflow.

The local store defaults to `~/.local/share/aiw`; `AIW_HOME` overrides it. `AIW_GIT` can select a working Git executable. Local paths belong in the machine registry, never in `aiw.yaml`.

For a new control plane integration, have the control plane create the Agent Repo worktree first. AIW's later `change materialize` command works inside that worktree and does not create it.
