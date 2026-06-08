# AIW Handoff

更新时间：2026-06-08

## 当前活动

优化默认 cmux layout：去掉底部独立 diff pane，保留底部单个 lazygit pane，并继续通过 lazygit overlay 使用 delta 渲染 diff。

## 本次已处理

- `src/layout.mjs` 的默认 layout 已从四 pane 调整为三 pane：

```text
+----------------------+----------------------+
| Files                | Agent                |
| aiw files            | codex/claude/etc.    |
+----------------------+----------------------+
| Git                                         |
| aiw git                                     |
+--------------------------------------------+
```

- `aiw diff` 命令仍保留；它不再是默认 workspace 的独立 pane。
- `src/deps.mjs` 的 `layout` / `cmux-new` / `init` gate 已不再把 `cmux-git-diff` 当作默认 workspace 必需项。
- 配置了 `git.lazygit_config` 时，`layout` / `cmux-new` / `init` / `git` gate 都会要求 `delta`，保证底部 lazygit pane 不是原生难读 diff。
- `README.md`、`README.zh-CN.md`、`skills/aiw-reference/SKILL.md`、`skills/aiw-init/SKILL.md` 已同步新默认模板和 gate 说明。
- 新增 `docs/2026-06-08-layout-template.md` 记录本次默认模板决策；更早的日期文档保持历史快照，不直接改写。

## 验证结果

- `node bin/aiw layout --agent codex --print-json` 通过，输出为上方 Files/Agent、底部 Git 的三 pane layout。
- `node bin/aiw doctor --gate layout --agent codex --json` 通过，gate satisfied 包含 `lazygit` 和 `delta`。
- `node bin/aiw doctor --gate cmux-new --agent codex --json` 通过，gate satisfied 包含 `lazygit` 和 `delta`，不依赖 `cmux-git-diff`。
- `node bin/aiw doctor --gate init --agent codex --json` 通过，gate satisfied 包含 `lazygit` 和 `delta`。
- `node bin/aiw doctor --gate git --json` 通过，gate satisfied 为 `lazygit`、`delta`。
- `ruby -ryaml -e 'YAML.load_file("config/lazygit-delta.yml")'` 通过。
- `npm run check` 通过；npm 仍会输出本机 npmrc 的 `always-auth` / `email` / `home` unknown config warning，但不影响检查。
- `git diff --check` 通过。

## 当前阻塞

- 无。

## 后续建议

- 如需进一步调高度，可以调整 `src/layout.mjs` 顶层 `split`；本次保留原来的 `0.56`，只改变底部 pane 结构。
