# Scratch Sessions - 2026-06-09

## 背景

Codex App 支持不绑定项目启动会话，并把对应工作区按日期和会话放在 `~/Documents/Codex` 下。AIW 新增对应的终端原生入口，但不做智能目录命名；默认使用本地日期和时间戳/uuid 组织。

## 行为

- 新入口：
  - `aiw cmux scratch`
  - `aiw scratch`
  - `aiw session`
- 默认根目录：`paths.sessions = "~/Documents/aiw"`。
- 默认目录结构：`~/Documents/aiw/YYYY-MM-DD/HHMMSS-<uuid8>`。
- 可通过 `--id <id>` 或第一个非 agent 位置参数指定会话 ID。
- 可通过 `--root <path>` 临时覆盖会话根目录。
- Scratch session 不要求当前目录是 Git repo，不调用 Worktrunk，不进入 `workspace done/gc` 生命周期。
- Scratch cmux layout 只有两个 pane：

```text
+----------------------+----------------------+
| Files                | Agent                |
| aiw files            | codex/claude/etc.    |
+----------------------+----------------------+
```

## Init 集成

`aiw init` 新增：

```bash
aiw init --sessions-root ~/Documents/aiw
```

初始化会创建 sessions root，并在 cmux config 中加入：

```text
aiw-scratch-session -> aiw cmux scratch
```

该 action 进入 plus-button context menu，不改变默认 plus-button 的 `aiw-new-worktree` 主路径。

## 验证

```bash
npm run check
node bin/aiw doctor --gate scratch --agent codex --json
node bin/aiw doctor --gate session --agent codex --json
node bin/aiw cmux scratch --agent codex --root /private/tmp/aiw-sessions --id cmux-smoke --dry-run
node /Users/bytedance/Code/aiw/bin/aiw cmux scratch --agent codex --root /private/tmp/aiw-sessions --id nongit-cmux --dry-run
node bin/aiw init --cmux-scope code --code-root /private/tmp/aiw-init-scratch-test-20260609b/code --worktrees-root /private/tmp/aiw-init-scratch-test-20260609b/worktrees --sessions-root /private/tmp/aiw-init-scratch-test-20260609b/sessions --config-dir /private/tmp/aiw-init-scratch-test-20260609b/config --launcher "node /Users/bytedance/Code/aiw/bin/aiw" --yes --no-reload
git diff --check
```

结果：

- `scratch` / `session` gate 通过，依赖只包含 `cmux`、`yazi`、`nvim`、目标 agent。
- 非 Git 目录 `/private/tmp` 下的 `aiw cmux scratch --dry-run` 通过。
- init 临时写入的 cmux action 命令为 `node /Users/bytedance/Code/aiw/bin/aiw cmux scratch`。
- 临时 init 目录已清理。
