# AIW Handoff

更新时间：2026-06-02

## 当前活动

修复 `aiw done` 在业务仓库中触发 Worktrunk fallback squash message，导致 Conventional Commit hook 拒绝的问题。当前工作区仍包含上一轮 npm publish 预备改动，尚未提交。

## 本次已处理

- 新增 `aiw commit-message` 命令：从 stdin 或 `--prompt` 读取 prompt，调用 AIW 当前 commit agent，只把清理后的 commit message 输出到 stdout，不执行 `git commit`。
- `aiw workspace done` / `aiw done` 在需要 Worktrunk squash commit message 且 Worktrunk 未配置生成器时，会为本次 `wt merge` 注入：

```bash
WORKTRUNK_COMMIT__GENERATION__COMMAND="<aiw> commit-message --agent <agent>"
```

- `aiw done --agent <name>` 现在用于选择注入给 Worktrunk 的 AIW commit agent；该参数不会透传给 `wt merge`。
- 如果用户已配置 `WORKTRUNK_COMMIT__GENERATION__COMMAND` 或 Worktrunk config 里已有 `[commit.generation] command`，AIW 不覆盖。
- `--no-squash` / `--no-commit` 不需要 squash message，AIW 不会强制检查 commit agent。
- 已更新 `README.md`、`README.zh-CN.md` 和 `skills/aiw-reference/SKILL.md` 的 `done` 说明。
- 上一轮 npm publish 预备改动仍在：`package.json` 已是 `0.1.1`，README 的 skills 安装示例已改为 `KiritoKing/aiw-cli`。
- 只读检查了用户报错的业务 worktree `/Users/bytedance/Code/cjpay_promotion_lynx_next.fix-return-state-mutation`：失败后 10 个文件全部 staged，未见 unstaged diff；可手工提交一个合规 Conventional Commit 后用 `aiw done dev --no-squash` 继续。

## 验证结果

- `npm run check` 通过。
- 临时 AIW config + mock agent 验证 `aiw commit-message` 的 stdin -> stdout 行为通过，输出 `fix: generated squash message`。
- `npm_config_cache=/private/tmp/aiw-npm-cache npm pack --dry-run --registry=https://registry.npmjs.org/` 通过，tarball 包含更新后的 `skills/aiw-reference/SKILL.md`、`src/commit.mjs`、`src/workspace.mjs` 等文件。
- `node bin/aiw workspace --help` 通过，help 已显示 `done [target] [--agent name] [--no-close-cmux]`。

## 当前阻塞

- 上一轮 npm publish 仍阻塞在公网 npm 登录/权限：`npm whoami --registry=https://registry.npmjs.org/` 返回 401；`@chlrc/aiw@0.1.1` 尚未发布。
- 完成公网 npm 登录并确认 `@chlrc` scope 权限后，可以发布当前 `0.1.1`：

```bash
npm publish --access public --registry=https://registry.npmjs.org/
```
