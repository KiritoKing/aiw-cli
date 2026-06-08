# AIW Handoff

更新时间：2026-06-08

## 当前活动

默认 cmux layout 已优化并推送；当前只剩 npm public registry 发布权限阻塞。

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
- 代码已提交并推送到 `origin/master`：
  - `46ec329 feat(layout): simplify default workspace git pane`

## 验证结果

- `node bin/aiw layout --agent codex --print-json` 通过，输出为上方 Files/Agent、底部 Git 的三 pane layout。
- `node bin/aiw doctor --gate layout --agent codex --json` 通过，gate satisfied 包含 `lazygit` 和 `delta`。
- `node bin/aiw doctor --gate cmux-new --agent codex --json` 通过，gate satisfied 包含 `lazygit` 和 `delta`，不依赖 `cmux-git-diff`。
- `node bin/aiw doctor --gate init --agent codex --json` 通过，gate satisfied 包含 `lazygit` 和 `delta`。
- `node bin/aiw doctor --gate git --json` 通过，gate satisfied 为 `lazygit`、`delta`。
- `ruby -ryaml -e 'YAML.load_file("config/lazygit-delta.yml")'` 通过。
- `npm run check` 通过；npm 仍会输出本机 npmrc 的 `always-auth` / `email` / `home` unknown config warning，但不影响检查。
- `git diff --check` 通过。
- `npm view @chlrc/aiw version --registry=https://registry.npmjs.org/` 返回 `0.1.0`，因此本地 `0.1.1` 版本号未撞线上版本。
- `npm pack --dry-run --registry=https://registry.npmjs.org/` 通过，包内容为 25 个文件，版本 `@chlrc/aiw@0.1.1`。
- `npm publish --access public --registry=https://registry.npmjs.org/` 未成功：registry 对 `PUT https://registry.npmjs.org/@chlrc%2faiw` 返回 404，并提示可能无权限；此前 `npm whoami --registry=https://registry.npmjs.org/` 返回 401。

## 当前阻塞

- npm public registry 未登录或当前账号没有 `@chlrc/aiw` 发布权限。线上仍是 `@chlrc/aiw@0.1.0`，`0.1.1` 尚未发布。

## 后续建议

- 先执行 `npm login --registry=https://registry.npmjs.org/`，确认 `npm whoami --registry=https://registry.npmjs.org/` 有返回账号。
- 如果 `@chlrc` scope 权限还未配置，需要确认该账号有发布 `@chlrc/aiw` 的权限。
- 权限确认后重新运行：

```bash
npm publish --access public --registry=https://registry.npmjs.org/
```
