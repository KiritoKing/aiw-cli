# AIW Workstation Runtime Migration - 2026-06-17

## 背景

AIW 早期以 cmux 作为主入口和默认 UI 承载层。随后一版方案尝试把 UI 建模为 `headless/modern` 加具体 implementation。实际使用后，用户反馈在 Ghostty 等普通现代终端中直接拉起 tmux 已经足够自然，继续暴露 backend 选择会增加不必要的配置面。

本次调整把 workstation 收敛为 runtime 自动判断：

- 普通终端、Ghostty、SSH、无 GUI 开发机：统一使用 tmux。
- cmux runtime：只有当前进程运行在 cmux 内，且 `cmux` CLI 可用时，使用 cmux。

因此 AIW 只支持两种 runtime implementation：`tmux` 和 `cmux`。

## 配置策略

不再要求用户配置 workstation backend。旧版本可能存在：

```toml
[workstation]
mode = "modern"
implementation = "cmux"
```

这个 section 现在是 legacy。AIW 不再读取它，也不再提供单独迁移命令；用户可以手工删除这段配置，但保留它也不会影响 runtime detection。

## 命令策略

主入口：

```bash
aiw new
aiw scratch
```

兼容入口保留：

```bash
aiw cmux-new
aiw cmux scratch
```

兼容入口只用于旧脚本和旧 cmux action 过渡。新 README、AGENTS、skills 和 `aiw init` 生成的 cmux actions 都应使用 `aiw new` / `aiw scratch`。

## 依赖门禁

`src/workstation.mjs` 返回 runtime UI 依赖，`src/deps.mjs` 区分 blocking 和 recommended：

- `tmux`：blocking dependency，`init/new/layout/scratch/scratch-resume` 都必须通过。
- `cmux`：recommended dependency，`doctor` 和 init preflight 以 warning 呈现。

缺少 `cmux` 时，普通工作流仍可用 tmux。真实执行 `aiw init` 时，用户需要 `--yes` 或交互确认后才能跳过 cmux recommended dependency。

## Layout 和 Adapter

`src/layout.mjs` 维护中立 layout model：

- project：Files + Agent 顶部，Git 底部。
- scratch：Files + Agent。

Adapter 由 `src/workstation.mjs` 执行：

- cmux adapter：把 model 转为 cmux workspace JSON 并调用 `cmux new-workspace`。
- tmux adapter：创建 tmux session 和 panes；创建或复用 session 时写入 `@aiw_managed`、`@aiw_cwd`、`@aiw_kind`、`@aiw_name` session options。

Ghostty 不再是 AIW implementation。它只是普通终端环境之一，AIW 在其中默认创建/attach tmux session。

## Init 策略

`aiw init` 不再询问 workstation backend。

```bash
npx @chlrc/aiw init --dry-run --yes
npx @chlrc/aiw init --cmux-scope home --yes
npx @chlrc/aiw init --cmux-scope none --yes
```

如果 `cmux` 可用，init 仍可注册 cmux actions。生成的 action 命令必须使用 `aiw new` / `aiw scratch`，不要重新引入 `aiw cmux-new` 主路径。

## Workspace 信号

workspace 表格列为 `UI`。JSON 记录包含：

- `open`
- `uiImplementation`
- `uiRef`

旧字段 `cmux` 暂时保留为兼容信号。`done` 的主选项是 `--no-close-ui`，`--no-close-cmux` 仅作为兼容别名保留。

tmux runtime 只把带 `@aiw_managed=1` 的 session 视为 AIW UI。这样 workspace list/gc/done/remove 可以统一管理 AIW 自己打开的 tmux session，同时避免误杀用户手工创建的普通 tmux session。

scratch session 使用同一套 UI metadata。`scratch list` 暴露 `open`、`uiImplementation`、`uiRef`，`scratch close` 关闭对应 UI，但不会删除 scratch 目录或 `.aiw-session.json`。

`scratch gc --tmux` 用于管理长期未使用的 tmux scratch session。它只处理 AIW-managed、`kind=scratch`、位于当前 scratch root 下、未 attach 且超过 stale 阈值的 tmux session；`--apply` / `--yes` 才会真正关闭，目录和 `.aiw-session.json` 保留。

## 验证清单

```bash
npm run check
node bin/aiw --help
node bin/aiw doctor --gate new --agent codex --json
node bin/aiw doctor --gate layout --agent codex --json
node bin/aiw layout --agent codex --dry-run
node bin/aiw scratch --agent codex --root /private/tmp/aiw-sessions --id smoke --dry-run
```
