# AIW 默认 Layout 模板调整

日期：2026-06-08

## 结论

默认 cmux layout 从四 pane 调整为三 pane：

```text
+----------------------+----------------------+
| Files                | Agent                |
| aiw files            | codex/claude/etc.    |
+----------------------+----------------------+
| Git                                         |
| aiw git                                     |
+--------------------------------------------+
```

## 原因

底部独立 `aiw diff --watch` pane 对默认工作流价值不高。日常 review、stage hunk、查看 diff 都可以在 lazygit 内完成，因此底部保留一个横跨全宽的 lazygit pane 更直接。

delta 仍是默认工作流的必要能力：`aiw git` 通过 `config/lazygit-delta.yml` 加载 lazygit overlay，使用 delta 渲染 lazygit 内的 diff，避免回到原生难读 diff。

## 依赖门禁

- `layout` / `cmux-new` / `init` 不再要求 `cmux-git-diff` 或 `delta` 二选一。
- 当配置了 `git.lazygit_config` 时，`layout` / `cmux-new` / `init` / `git` 都要求 `delta`。
- `aiw diff` 命令仍保留；它的独立 gate 继续使用 `cmux-git-diff`，缺失时 fallback 到 `git diff | delta`。

## 验证

```bash
node bin/aiw layout --agent codex --print-json
node bin/aiw doctor --gate layout --agent codex --json
node bin/aiw doctor --gate cmux-new --agent codex --json
node bin/aiw doctor --gate init --agent codex --json
node bin/aiw doctor --gate git --json
ruby -ryaml -e 'YAML.load_file("config/lazygit-delta.yml")'
npm run check
git diff --check
```
