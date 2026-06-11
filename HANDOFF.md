# AIW Handoff

更新时间：2026-06-11

## 当前活动

已修复 `aiw ws` 在包含坏 worktree / prunable worktree 的仓库里崩溃的问题。

## 本次已处理

- 复现命令：

```bash
cd /Users/bytedance/Code/marketing-x.feat-yunti-withdraw
node /Users/bytedance/Code/aiw/bin/aiw ws
```

- 根因：
  - `wt list --format json` 会返回 prunable worktree，例如 `/Users/bytedance/Code/wt-feat/openspec-prd-test-2`。
  - AIW 后续用 `tryCapture("git", ["status", "--short"], { cwd })` 探测 dirty 状态。
  - 当 `cwd` 不存在时，`spawnSync` 返回启动级错误，`stdout` / `stderr` 为 `undefined`。
  - 旧版 `tryCapture()` 直接读取 `result.stdout.trim()`，触发 `Cannot read properties of undefined (reading 'trim')`。

- 修复：
  - `src/run.mjs` 现在会先归一化 `spawnSync` 的 `stdout`、`stderr` 和 `error`。
  - `tryCapture()` 对启动级错误返回 `{ ok: false, status: 1, stdout: "", stderr: "<error>" }`。
  - `capture()` 也会把 `spawnSync` 的 `error.message` 纳入失败消息，避免同类缺失输出问题。

## 验证结果

```bash
npm run check
node /Users/bytedance/Code/aiw/bin/aiw ws
node /Users/bytedance/Code/aiw/bin/aiw ws list --json
```

- `npm run check` 通过。
- 在 `/Users/bytedance/Code/marketing-x.feat-yunti-withdraw` 下，`aiw ws` 已正常输出 workspace 表。
- `aiw ws list --json` 已正常输出 7 条 workspace 记录。
- 本机 shell 仍会输出 `fnm_multishells ... Operation not permitted` 噪音；命令主体成功。

## 当前 Git 状态

- 当前分支：`master`。
- 本次改动文件：
  - `src/run.mjs`
  - `HANDOFF.md`

## 后续建议

- 如需让全局 `aiw` 立即使用这次 checkout 的修复，确认 `/Users/bytedance/.local/bin/aiw` 仍指向本地 launcher；否则执行项目既有安装流程。
