# AIW Handoff

更新时间：2026-06-09

## 当前活动

新增非项目 scratch session 入口已完成本地实现与验证；用户要求按新配置重新初始化 cmux，但当前 Codex 文件沙箱阻止写入 `~/Documents/aiw` / `~/.config/cmux`，真实初始化未完成。

## 本次已处理

- 新增 `aiw cmux scratch`，并保留短别名：
  - `aiw scratch`
  - `aiw session`
- Scratch session 默认创建在 `paths.sessions` 下，默认值为 `~/Documents/aiw`。
- 默认目录结构为：

```text
~/Documents/aiw/YYYY-MM-DD/HHMMSS-<uuid8>
```

- 支持：
  - `--id <id>` 或位置参数指定 session id。
  - `--root <path>` 临时覆盖 session root。
  - `--dry-run` 只打印目录和 cmux 命令，不创建目录。
- Scratch cmux layout 是两 panel：

```text
+----------------------+----------------------+
| Files                | Agent                |
| aiw files            | codex/claude/etc.    |
+----------------------+----------------------+
```

- Scratch 不要求当前目录是 Git repo，不调用 Worktrunk，不打开 Git pane，不参与 `workspace done/gc`。
- 新增 `scratch` / `session` dependency gate，只要求 `cmux`、`yazi`、`nvim`、目标 agent。
- `aiw init` 新增 `--sessions-root`，并在 cmux config 中加入：

```text
aiw-scratch-session -> aiw cmux scratch
```

- `README.md`、`README.zh-CN.md`、`skills/aiw-reference/SKILL.md`、`skills/aiw-init/SKILL.md` 已同步。
- 新增历史记录：`docs/2026-06-09-scratch-sessions.md`。

## 验证结果

- `npm run check` 通过；npm 仍输出本机 npmrc 的 `always-auth` / `email` / `home` unknown config warning，不影响检查。
- `node bin/aiw doctor --gate scratch --agent codex --json` 通过。
- `node bin/aiw doctor --gate session --agent codex --json` 通过。
- `node bin/aiw cmux scratch --agent codex --root /private/tmp/aiw-sessions --id cmux-smoke --dry-run` 通过，输出为两 panel layout。
- 在非 Git 目录 `/private/tmp` 下运行 `node /Users/bytedance/Code/aiw/bin/aiw cmux scratch --agent codex --root /private/tmp/aiw-sessions --id nongit-cmux --dry-run` 通过。
- 临时运行 `aiw init --cmux-scope code ... --sessions-root ... --no-reload` 通过，并确认写出的 cmux action 命令为 `node /Users/bytedance/Code/aiw/bin/aiw cmux scratch`。
- `git diff --check` 通过。
- 临时 init 目录已清理。

## 最新执行状态

- `node bin/aiw init --dry-run --yes --cmux-scope home` 通过，计划为：
  - 保留现有 `~/.config/aiw` 配置文件。
  - 创建 `~/Documents/aiw`。
  - merge `~/.config/cmux/cmux.json`，并备份为 `~/.config/cmux/cmux.json.20260609T062835.bak`。
  - 写入 `aiw-scratch-session -> aiw cmux scratch`。
- `node bin/aiw init --yes --cmux-scope home` 未成功，失败点：

```text
EPERM: operation not permitted, mkdir '/Users/bytedance/Documents/aiw'
```

- 原因是当前 Codex 会话文件沙箱只允许写 `/Users/bytedance/Code/aiw` 和临时目录，不能写用户 home 下的 Documents / cmux config。
- 需要在用户本机终端直接运行：

```bash
cd /Users/bytedance/Code/aiw
node bin/aiw init --yes --cmux-scope home
```

## 后续建议

- 在本机终端完成 cmux 初始化后，运行 `cmux reload-config` 或确认 `aiw init` 自动 reload 成功。
- 用户确认体验后，提交并发布新版本。
