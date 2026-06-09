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

## Push / Publish 状态

- Scratch 改动已在本地提交：

```text
017ad03 feat(cmux): add scratch sessions
```

- 当前分支状态：

```text
master...origin/master [ahead 1]
```

- `npm run check` 通过。
- `npm pack --dry-run --cache /private/tmp/aiw-npm-cache --registry=https://registry.npmjs.org/` 通过，包为 `@chlrc/aiw@0.1.1`，package size `55.1 kB`，total files `25`。
- `git push origin master` 未成功，失败点：

```text
Could not resolve host: github.com
```

- `npm view @chlrc/aiw version --registry=https://registry.npmjs.org/` 未成功，失败点：

```text
getaddrinfo ENOTFOUND registry.npmjs.org
```

- `npm whoami --registry=https://registry.npmjs.org/` 未成功，同样是 `getaddrinfo ENOTFOUND registry.npmjs.org`。
- `npm publish --access public --registry=https://registry.npmjs.org/ --cache /private/tmp/aiw-npm-cache` 已执行，完成本地 tarball 生成后发布失败，失败点：

```text
getaddrinfo ENOTFOUND registry.npmjs.org
```

- 当前 Codex 执行环境不能自行提权或解除网络/DNS 限制；push 和 publish 需要在有网络的本机终端执行：

```bash
cd /Users/bytedance/Code/aiw
git push origin master
npm whoami --registry=https://registry.npmjs.org/
npm view @chlrc/aiw version --registry=https://registry.npmjs.org/
npm publish --access public --registry=https://registry.npmjs.org/ --cache /private/tmp/aiw-npm-cache
```

## 后续建议

- 在本机终端完成 cmux 初始化后，运行 `cmux reload-config` 或确认 `aiw init` 自动 reload 成功。
- 在有外网 DNS 的终端完成 `git push origin master` 和 `npm publish`。

## 已发现问题：cmux launcher 包名错误

- 用户在 cmux 里触发 `npx aiw cmux-new --pick-repo` 时，npx 命中了 npm 上另一个未 scoped 包：

```text
Need to install the following packages:
aiw@1.0.0
```

- 根因：`src/init.mjs` 的默认 `launcher` 是 `npx aiw`，而本项目的正确包名是 `@chlrc/aiw`。
- 已在代码中修复默认值为：

```text
npx --yes @chlrc/aiw
```

- 当前实际 `~/.config/cmux/cmux.json` 仍含旧命令：

```text
npx aiw cmux-new
npx aiw cmux-new --pick-repo
npx aiw cmux-new --local
npx aiw cmux scratch
```

- 当前 Codex 沙箱不能写 `~/.config/cmux/cmux.json`，执行 `node bin/aiw init --yes --cmux-scope home` 失败于：

```text
EPERM: operation not permitted, copyfile '/Users/bytedance/.config/cmux/cmux.json' -> '/Users/bytedance/.config/cmux/cmux.json.20260609T084710.bak'
```

- 需要在用户本机终端运行本地 checkout 的修复版 init：

```bash
cd /Users/bytedance/Code/aiw
node bin/aiw init --yes --cmux-scope home
```
