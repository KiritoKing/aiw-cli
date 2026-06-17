# AIW Handoff

更新时间：2026-06-17

## 当前活动

正在把 AIW workstation runtime 收敛为自动判断：

- 主入口仍是 `aiw new`。
- `cmux-new` 和 `aiw cmux scratch` 保留为兼容别名。
- 用户不再选择 backend；AIW 根据 runtime 自动选择 UI。
- 默认所有普通终端、Ghostty、SSH、无 GUI 开发机都走 tmux。
- 只有当前进程在 cmux runtime 内，且 `cmux` CLI 可用时，才走 cmux 命令。
- `aiw migrate` 用于移除旧 `[workstation]` 选择配置。

## 当前实现边界

Runtime 选择：

- `tmux`：必需依赖，默认 workstation runtime。
- `cmux`：推荐依赖，只在 cmux runtime 内使用。

布局模型：

- project：顶部 Files + Agent，底部 Git。
- scratch：Files + Agent。
- cmux adapter 继续输出 cmux workspace JSON。
- tmux adapter 创建 tmux session 和 panes。

依赖门禁：

- `doctor --gate new/layout/scratch` 必须要求 `tmux`。
- `cmux` 只作为 recommended missing 输出。
- `aiw init` 在真实写入时，如果缺少 cmux，需要 `--yes` 或交互确认才能跳过。

## 待验证命令

代码改动后至少验证：

```bash
npm run check
node bin/aiw --help
node bin/aiw migrate --dry-run --json
node bin/aiw doctor --gate new --agent codex --json
node bin/aiw doctor --gate layout --agent codex --json
node bin/aiw layout --agent codex --dry-run
node bin/aiw scratch --agent codex --root /private/tmp/aiw-sessions --id smoke --dry-run
```

迁移验证建议使用 `/private/tmp` 下的临时 config dir，覆盖：

- 旧 `[workstation]` 配置 dry-run JSON。
- 实际迁移创建 backup 并移除 `[workstation]`。
- 没有 `[workstation]` 时默认 no-op。

## 兼容边界

- `aiw cmux-new` 仍可用，但新文档和生成配置应使用 `aiw new`。
- `aiw cmux scratch` 仍可用，但新文档和生成配置应使用 `aiw scratch`。
- `--no-close-cmux` 仍可解析，但主选项是 `--no-close-ui`。
- JSON workspace 记录保留旧 `cmux` 字段作为兼容信号，同时保留 `open` 和 `uiImplementation`。

## 环境注意

- 本机 shell 可能输出 `fnm_multishells ... Operation not permitted` 噪音；命令主体成功时忽略。
- 历史 dated docs 保留旧 cmux-first 叙事作为历史记录，不要为了本次重构改写旧日志。
