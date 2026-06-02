# AIW Handoff

更新时间：2026-06-02

## 当前活动

`aiw cmux-new` 已补充“从已有 branch 创建新工作区”的入口，当前改动尚未提交。

## 本次已处理

- 已确认原行为属实：
  - 不存在的 `--branch` 或交互创建新分支时，原逻辑固定使用当前 `HEAD` 作为 base。
  - 选择已有 branch 时，原逻辑只执行 `wt switch <branch>` 打开或切换该 branch。
- 新增非交互入口：
  - `aiw cmux-new --branch <new-branch> --base <base-branch> --agent <agent>`
  - `--from <base-branch>` 是 `--base <base-branch>` 的 alias。
- 新增交互入口：
  - `Select worktree` 中增加 `Create new branch from existing branch...`。
  - 选择后先选 base branch，再输入新 branch。
- 创建命令现在会根据选择生成：
  - 从当前 `HEAD` 创建：`wt switch --create <branch> --base @ -x "aiw layout --agent <agent>"`
  - 从已有 branch 创建：`wt switch --create <branch> --base <base> -x "aiw layout --agent <agent>"`
- 创建时记录的 workspace target 会使用指定 base branch；未指定 base 时继续使用当前 branch。
- 已更新 `README.md` 和 `README.zh-CN.md` 的 `cmux-new` 行为说明。

## 验证结果

- `npm run check` 通过。
- 在 `/private/tmp/aiw-cmux-base-test` 创建一次性 Git repo 后验证 dry-run：
  - `node bin/aiw cmux-new --repo /private/tmp/aiw-cmux-base-test --branch feat/from-develop --base develop --agent codex --dry-run`
  - 输出包含 `wt switch --create feat/from-develop --base develop -x ...`。
- 边界验证：
  - `--branch develop --base main` 在目标 branch 已存在时提前报错，退出码 `2`。
  - `--branch feat/missing-base --base missing-branch` 在 base 不存在时提前报错，退出码 `4`。
  - `--from develop` alias 会生成同样的 `--base develop` Worktrunk 命令。

## 需要特别注意

- 这次没有修改历史 dated work log。
- 验证只执行了 Worktrunk dry-run 输出检查，没有创建真实 worktree。
