# AIW Handoff

更新时间：2026-06-03

## 当前活动

合并 `aiw done` 的 Worktrunk commit-message bridge、失败 retry/rollback，以及日常 alias 增强。

## 本次已处理

- `aiw commit-message` 命令保留：从 stdin 或 `--prompt` 读取 prompt，调用 AIW 当前 commit agent，只输出清理后的 commit message，不执行 `git commit`。
- `aiw workspace done` / `aiw done` 在需要 Worktrunk squash commit message 且 Worktrunk 未配置生成器时，会为本次 `wt merge` 注入：

```bash
WORKTRUNK_COMMIT__GENERATION__COMMAND="<aiw> commit-message --agent <agent>"
```

- `aiw done --agent <name>` 用于选择注入给 Worktrunk 的 AIW commit agent；该参数不会透传给 `wt merge`。
- 如果用户已配置 `WORKTRUNK_COMMIT__GENERATION__COMMAND` 或 Worktrunk config 里已有 `[commit.generation] command`，AIW 不覆盖。
- `--no-squash` / `--no-commit` 不需要 squash message，AIW 不会强制检查 commit agent。
- `aiw workspace done` / `aiw done` 新增外层 retry：
  - 默认读取 `commit.retries`，当前默认 3。
  - 支持 `--retries <n>` / `--retries=<n>` 覆盖。
  - 每次 `wt merge` 失败后恢复 source worktree、target branch 和 Worktrunk `refs/wt-backup/<branch>`。
  - 最终失败时仍保持 source worktree clean，避免留下 Worktrunk squash/rebase 的中间脏状态。
- `done` 在进入 `wt merge` 前会检查目标分支对应 worktree；如果目标 worktree dirty，会直接拒绝并提示先 clean/stash。
- 补齐 alias：
  - `aiw ls`、`aiw als`、`aiw ws ls`、`aiw ws als` -> workspace list。
  - `aiw new` 和 `aiw cmux new` -> `aiw cmux-new`。
  - `aiw new <agent>` 的位置参数解析已与 `aiw cmux-new <agent>` 对齐。
- 已合并 rebase 冲突，冲突文件包括：
  - `src/workspace.mjs`
  - `src/cli.mjs`
  - `README.md`
  - `README.zh-CN.md`
  - `skills/aiw-reference/SKILL.md`
  - `HANDOFF.md`
- 上一轮 npm publish 预备改动仍在：`package.json` 已是 `0.1.1`，README 的 skills 安装示例已改为 `KiritoKing/aiw-cli`。

## 验证结果

- `npm run check` 通过。
- `git diff --check` 通过。
- 临时 AIW config + mock agent 验证 `aiw commit-message` 的 stdin -> stdout 行为通过，输出 `fix: generated squash message`。
- 临时 Git worktree 验证 `aiw done main --retries 2 --no-close-cmux`：
  - commit-msg hook 故意失败时，实际执行 2 次 merge attempt。
  - 最终失败后 source HEAD、target HEAD 均恢复到执行前。
  - source/target `git status --porcelain` 均为空。
  - Worktrunk backup ref 未残留。
- 临时 Git worktree 验证目标 worktree dirty：
  - 命令在进入 merge 前以 exit 5 拒绝。
  - 未出现 merge attempt。
- 临时 Git worktree 验证成功路径：
  - 正常 1 次 attempt 后 merge 到 target。
  - target worktree 保持 clean。
- alias smoke test 通过：
  - `aiw ls --json`
  - `aiw ALS --json`
  - `aiw ws als --json`
  - `aiw new --repo <repo> --branch <branch> --dry-run`
  - `aiw cmux new --repo <repo> --branch <branch> --dry-run`

## 当前阻塞

- 上一轮 npm publish 仍阻塞在公网 npm 登录/权限：`npm whoami --registry=https://registry.npmjs.org/` 返回 401；`@chlrc/aiw@0.1.1` 尚未发布。
- 完成公网 npm 登录并确认 `@chlrc` scope 权限后，可以发布当前 `0.1.1`：

```bash
npm publish --access public --registry=https://registry.npmjs.org/
```

## 后续建议

- 如果后续要进一步降低 `done` 的黑盒程度，可以把 retry/rollback 的临时仓库场景固化成脚本化测试。
- Worktrunk 成功 merge 后 worktree removal 是后台动作；如果未来要在 AIW 中做强校验，需要额外等待/轮询 Worktrunk cleanup 结果。
