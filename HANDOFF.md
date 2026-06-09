# AIW Handoff

更新时间：2026-06-09

## 当前活动

Scratch session 管理能力已实现并验证；真实 cmux 配置已重刷为本地 checkout launcher。

## 本次已处理

- 新增 scratch session 管理入口：
  - `aiw scratch list`
  - `aiw scratch resume`
  - `aiw cmux scratch resume`
- 新建 scratch session 时写入 `.aiw-session.json`，记录：
  - 创建时间
  - agent
  - session id
  - 第一条消息
  - session path
- `aiw scratch resume` 扫描 `paths.sessions`，用 fzf TUI 展示并搜索：

```text
时间    session id    第一条消息    路径
```

- 支持按日期、session id、第一条消息内容、路径片段模糊匹配。
- 支持非交互直开：

```bash
aiw scratch resume --id <session-id>
```

- 新增 `scratch-resume` dependency gate，要求 `cmux`、`yazi`、`nvim`、`fzf` 和目标 agent。
- `aiw init` 现在会注册两个 scratch 相关 cmux action：

```text
aiw-scratch-session -> aiw cmux scratch
aiw-scratch-resume  -> aiw cmux scratch resume
```

- 修复 `aiw init` 默认 launcher：从错误的 `npx aiw` 改为 `npx --yes @chlrc/aiw`，避免命中 npm 上另一个 `aiw@1.0.0` 包。
- 本机真实 `~/.config/cmux/cmux.json` 已用本地 checkout launcher 重刷：

```text
node /Users/bytedance/Code/aiw/bin/aiw cmux-new
node /Users/bytedance/Code/aiw/bin/aiw cmux-new --pick-repo
node /Users/bytedance/Code/aiw/bin/aiw cmux-new --local
node /Users/bytedance/Code/aiw/bin/aiw cmux scratch
node /Users/bytedance/Code/aiw/bin/aiw cmux scratch resume
```

- `cmux reload-config` 已成功。
- `README.md`、`README.zh-CN.md`、`skills/aiw-reference/SKILL.md`、`skills/aiw-init/SKILL.md`、`docs/2026-06-09-scratch-sessions.md` 已同步。

## 验证结果

- `node bin/aiw scratch list --root /private/tmp/aiw-session-fixture --json` 通过。
- `node bin/aiw scratch list --root /private/tmp/aiw-session-fixture` 通过。
- `node bin/aiw scratch resume --root /private/tmp/aiw-session-fixture --id 142939-912bdf48 --agent codex --dry-run` 通过。
- `node bin/aiw doctor --gate scratch-resume --agent codex --json` 通过。
- `node bin/aiw init --dry-run --yes --cmux-scope home` 通过。
- `node bin/aiw init --yes --cmux-scope home --launcher "node /Users/bytedance/Code/aiw/bin/aiw"` 通过并 reload cmux。
- `npm run check` 通过；npm 仍输出本机 npmrc 的 `always-auth` / `email` / `home` unknown config warning，不影响检查。
- `git diff --check` 通过。
- `/private/tmp/aiw-session-fixture` 已清理。

## 当前 Git 状态

- 本地 `master` 领先 `origin/master` 3 个历史提交。
- 本次 session resume 改动尚未提交。

## 后续建议

- 提交本次改动。
- 网络可用后 push 并发布新版本。
