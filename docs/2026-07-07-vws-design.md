# AIW VWS 方案设计

日期：2026-07-07

## 背景

AIW 的核心不应该被定义成 agent interface。agent interface 可以由 Paseo、cmux、rmux、tmux 或未来其他界面承载。AIW 真正要沉淀的是 AI 协作工作流：从需求进入、工作区创建、上下文初始化、任务拆解、子任务执行、验收、合并到清理的工程流程。

在 cmux 时代，AIW 主要负责单仓 feature 开发流程：

```text
cd 到仓库
-> aiw new
-> 创建 worktree，拉起 cmux/tmux 工作区
-> 开始工作
-> aiw done
-> 合入分支，关闭工作区
```

在 Paseo 时代，本地 GUI 流程会变成：

```text
Paseo 选择 workspace
-> 创建新 worktree/task
-> 在 Paseo 中推进任务
-> 必要时通过内置 terminal 调用 aiw commit、aiw done 等能力
```

headless 或云端流程仍然需要 AIW：

```text
aiw new / aiw vws new
-> 创建工作区
-> 通过 CLI 调度 Paseo 或其他后端发起任务
-> aiw done / aiw vws close
-> 合入或归档，清理工作区
```

因此，Paseo 更适合作为 AIW 工作流的新交互承载层，而不是 AIW 的替代品。AIW 需要继续负责多仓需求级工作区组织、分支生命周期、上下文文件协议和调度规则。

## VWS 定义

VWS 是 Virtual Workspace 的缩写，表示一个需求级虚拟工作区。

VWS 不是单纯的多 repo worktree 目录集合，而是一个需求协作运行时目录。它包含：

- 多个业务 repo 的需求级 dev worktree。
- 每个 repo、每个任务的 feature worktree。
- 需求上下文、任务表、跨仓契约、依赖关系和验收口径。
- leader agent 的文件化状态。
- worker agent 的调度记录和执行证据。
- 可 review 的跨仓 diff snapshot。

推荐目录结构：

```text
<vws-root>/
  .git/
  .gitignore
  .vws/
    vws.yaml
    requirement.md
    context.md
    plan.md
    tasks.yaml
    handoff.md
    dependencies.md
    contracts/
      api.md
      data.md
      rollout.md
    reviews/
      summary.md
      repo-status.yaml
      repo-a.patch
      repo-b.patch
    state/
      events.jsonl
      leader.md
      dispatch.yaml
    skills/
      init-context/
      dispatch/
      status-sync/
      verify-contract/
  worktrees/
    repo-a/
    repo-b/
    service-c/
    service-d/
  tasks/
    repo-a/
      task-login-ui/
    service-c/
      task-add-endpoint/
```

VWS 根目录本身应该是一个临时 Git repo，用于 review VWS 自身的上下文、计划、任务、契约和状态文件。业务仓库 worktree 不应该默认作为 submodule 或 subtree 纳入 VWS Git。它们仍然保持独立 Git 边界。

## 角色边界

```text
AIW VWS
  需求级工作流协议、文件状态、生命周期和调度工具

Paseo
  用户可见 control plane，承载 leader agent、状态跟进和移动端交互

Leader agent
  需求级协调者，负责理解需求、维护全局状态、拆任务、调度、验收和推进阻塞

Worker agent
  repo/task 级后台执行者，默认不可见或弱可见，只在对应 feature worktree 内工作

Repo-local AI infra
  每个业务仓库自己的 README、AGENTS、skills、测试命令和工程规范
```

Leader agent 不应该变成超级开发者。它像 tech lead，负责全局协调和契约检查，不直接包办所有代码修改。Worker agent 是后台 subagent，默认不需要作为用户可见的独立 AIW session，也不必进入 Paseo 或 cmux 的显式面板。只有当某个子任务需要人工 debug 时，才把 worker 暴露为 Paseo 可见 session 或打开 terminal。

## 核心流程

### 1. 创建 VWS

```bash
aiw vws new <name>
aiw vws new <name> --repo cfe=~/Code/a --repo bfe=~/Code/b --repo svc=~/Code/c
aiw vws new <name> --base master --pull
```

`aiw vws new` 支持 TUI 和 CLI 两种入口：

- 交互式命名需求。
- 交互式选择本地 repos。
- 校验 repo 路径和 Git 状态。
- 根据 `--base` 和 `--pull` 更新 source repo 的目标基线。
- 创建 VWS 根目录。
- 初始化 VWS Git repo。
- 为每个 repo 创建需求级 dev worktree。
- 写入 `.vws/` 文件协议。
- 注入 VWS skills 和 SOP。
- 拉起 Paseo workspace 和 leader agent。

创建完成后，用户主要进入 Paseo 工作，不再频繁手动调用 AIW。

### 2. 初始化业务上下文

进入 VWS 后，Paseo 自动拉起 leader agent。用户一般手动调用初始化 skill，并传入需求文档、PRD 链接、补充说明或验收口径。

Leader agent 需要把理解结果写入文件，而不是依赖上下文窗口：

- `.vws/requirement.md`
- `.vws/context.md`
- `.vws/plan.md`
- `.vws/tasks.yaml`
- `.vws/contracts/*`
- `.vws/state/events.jsonl`
- `.vws/state/leader.md`

所有重要行为必须 append 到 `events.jsonl`。上下文窗口只能作为缓存，不能作为事实源。

### 3. 调度子任务

```bash
aiw vws dispatch
aiw vws task start <task-id>
```

Leader agent 调用内置调度 skill，根据 `tasks.yaml` 拆分 repo/task 级任务，并通过 AIW VWS 调度能力启动 worker。

底层调度后端可以替换：

- Paseo：首选 control plane，适合移动端状态跟进。
- rmux：适合 headless 多 pane/session。
- cmux：历史 GUI runtime。
- tmux：基础 headless fallback。
- direct agent CLI：适合完全后台 worker。

Paseo 不代表每个 worker 都必须出现在 UI 中。更理想的方式是：

```bash
paseo run --detach --cwd tasks/<repo>/<task> \
  --label vws=<vws-id> \
  --label repo=<repo-key> \
  --label role=worker \
  "<task prompt>"
```

或者由 AIW 直接启动 provider CLI，并把 stdout、状态和证据写入 VWS 文件。

### 4. 文件化状态

Leader agent 和 AIW 工具之间通过 VWS 文件协议交接状态。

核心文件：

```yaml
# .vws/vws.yaml
id: pay-withdraw-v2
status: active
backend: paseo
base:
  default: master
repos:
  cfe:
    source: /Users/bytedance/Code/a
    dev_branch: vws/pay-withdraw-v2/dev
    dev_worktree: worktrees/cfe
    target_branch: master
  svc:
    source: /Users/bytedance/Code/c
    dev_branch: vws/pay-withdraw-v2/dev
    dev_worktree: worktrees/svc
    target_branch: master
```

```yaml
# .vws/tasks.yaml
tasks:
  cfe-login-ui:
    repo: cfe
    status: running
    base_branch: vws/pay-withdraw-v2/dev
    feature_branch: vws/pay-withdraw-v2/task/cfe-login-ui
    worktree: tasks/cfe/cfe-login-ui
    owner: worker
    dependencies:
      - svc-add-endpoint
```

```yaml
# .vws/state/dispatch.yaml
workers:
  cfe-login-ui:
    backend: paseo
    agent_id: 3cdcb4ca-0332-401e-8fae-8bc150cc159f
    repo: cfe
    task: cfe-login-ui
    status: running
    last_event_at: 2026-07-07T10:00:00+08:00
```

### 5. 任务完成与合入

每个开发迭代仍遵循 AIW feature dev 分支模式。

规则：

1. VWS 中每个 repo 的 dev worktree 是需求级集成分支，不直接开发。
2. 每个 repo 的每个任务都从 repo dev 分支拉新的 feature worktree。
3. Worker 只修改对应 task feature worktree。
4. 任务完成后合入 repo dev 分支。
5. repo dev 分支作为该需求在该 repo 的本地集成结果。

示例：

```text
repo-a:
  dev branch:     vws/pay-withdraw-v2/dev
  feature branch: vws/pay-withdraw-v2/task/login-ui

repo-c:
  dev branch:     vws/pay-withdraw-v2/dev
  feature branch: vws/pay-withdraw-v2/task/add-endpoint
```

任务完成命令：

```bash
aiw vws task done <task-id>
```

`task done` 做：

- 进入 task feature worktree。
- 按 repo-local 规则运行检查。
- 生成提交或要求 worker 先提交。
- 合入对应 repo dev 分支。
- 更新 `tasks.yaml` 和 `dispatch.yaml`。
- 清理或归档 task feature worktree。

`task done` 只合入 VWS dev，不合入主干。

### 6. 需求关闭

```bash
aiw vws close --cloud-merge
```

整个需求 close 后，本地只负责确认、归档和清理。最终合并在云端完成，例如通过 MR、CI、review gate 或内部发布流程。

`vws close` 要求：

- 所有 task 都已结束或显式标记为跳过。
- 所有 task feature 分支都已合入对应 repo dev。
- VWS dev worktree 没有未提交改动。
- 生成 `.vws/reviews/summary.md`。
- 生成 `.vws/reviews/repo-status.yaml`。
- 记录每个 repo 的 dev branch、target branch、commit SHA 和验证结果。
- 输出云端合并交付信息。
- 本地清理 VWS worktrees 和临时状态。

VWS close 不直接把 dev 合入 master。它只负责把本地需求级交付物整理好，交给云端完成最终合并。

## 基线同步

用户通常仍会在 terminal 发起任务，因为 terminal 更适合拉取分支、管理本地 worktree 和处理 Git 基线。

因此，Paseo 不替代 terminal 的基线管理。推荐职责：

```text
Terminal / AIW:
  选择 repo、fetch/pull master、创建 VWS、创建 dev worktree、维护基线

Paseo:
  leader agent、worker 调度、移动端跟进、日常协作
```

需要区分 source repo 和 VWS worktree：

```text
source repo:
  /Users/bytedance/Code/a
  用于 fetch/pull master，作为本地基线来源

vws dev worktree:
  <vws-root>/worktrees/a
  需求集成分支，不直接拉 master

task feature worktree:
  <vws-root>/tasks/a/task-login-ui
  子任务开发分支
```

命令：

```bash
aiw vws sync-base
aiw vws rebase-dev
aiw vws merge-base
```

`sync-base` 在 source repo 上执行 fetch/pull，不在 Paseo worker 的 feature worktree 里拉 master。

长周期需求需要同步主干时，leader agent 可以提醒用户或调用 AIW 工具执行：

```text
source repo fetch/pull
-> 更新 target branch
-> rebase/merge VWS dev branch
-> 检查每个 task feature branch 是否需要同步
```

## Review 设计

VWS 不是 Git monorepo。业务 repo 的代码不应该默认作为 submodule 或 subtree 放进 VWS repo。

差异：

- worktree：同一个 repo 的多个工作目录，适合 dev/task 分支开发。
- submodule：父 repo 记录外部 repo 的 commit 指针，适合 close/archive 阶段做版本锚点。
- subtree：把外部 repo 内容并入父 repo，侵入过强，不适合 VWS。

推荐方案：

```text
VWS 根目录是 Git repo
业务仓库使用 worktree
AIW 生成 reviews/*.patch 和 repo-status.yaml
必要时 close 阶段记录 submodule/gitlink 风格的 commit 锚点
```

命令：

```bash
aiw vws diff
aiw vws snapshot
```

生成：

```text
.vws/reviews/
  summary.md
  repo-status.yaml
  repo-a.patch
  repo-b.patch
  service-c.patch
```

这让用户可以 review VWS 自身变化，也可以 review 每个 repo 的代码 diff。

## Skills 与 SOP

`aiw vws new` 会在 VWS 目录下注入 VWS 专属 skills，而不是写入业务仓库。

初始 skills：

- `init-context`：基于 PRD/补充说明初始化需求上下文。
- `dispatch`：根据任务表创建 worker 任务。
- `status-sync`：同步 worker 状态和 repo 状态。
- `verify-contract`：检查跨仓契约是否被满足。
- `close-vws`：收尾、生成 review snapshot 和云端合并材料。

业务 repo 内的开发仍然高度依赖 repo 自身的 AI 基建：

- repo-local `AGENTS.md`
- repo-local skills
- README
- 测试命令
- lint/typecheck/build
- 业务约定

上层 VWS 不应该侵入或覆盖 repo-local 规则。

## Paseo 交互模型

推荐交接关系：

```text
用户在 terminal 运行 aiw vws new
-> AIW 创建 VWS 和 worktrees
-> AIW 打开 Paseo workspace
-> Paseo 拉起 leader agent
-> 用户后续主要在 Paseo 里工作
-> leader agent 调用 AIW VWS tools 调度后台 worker
```

用户不应该反复手动调用 `aiw vws dispatch`、`aiw vws status`、`aiw vws task done`。这些命令主要作为 leader agent 可调用的工具存在。

Paseo 补齐了之前自定义 agent interface 缺失的移动端能力：

- 手机跟进 agent 状态。
- 手机继续对话。
- 看日志和任务进展。
- 必要时远程发起指令。

飞书 bot 只能做较差的消息桥，不适合作为长期主交互面。

## 命令草案

```bash
# 创建/打开
aiw vws new [name] [--repo key=path] [--base branch] [--pull] [--backend paseo|rmux|cmux|tmux|direct]
aiw vws open [name]

# 上下文
aiw vws init-context [--doc path-or-url] [--note text]
aiw vws status [--json]

# 任务
aiw vws task create <id> --repo <key> --prompt <text>
aiw vws task start <id>
aiw vws task done <id>
aiw vws task abort <id>

# 调度
aiw vws dispatch [--backend paseo|rmux|cmux|tmux|direct]
aiw vws workers [--json]

# 基线
aiw vws sync-base
aiw vws rebase-dev [repo]
aiw vws merge-base [repo]

# Review
aiw vws diff
aiw vws snapshot

# 验收/收尾
aiw vws verify
aiw vws close --cloud-merge
aiw vws gc
```

## MVP 切分

### Phase 1：VWS 文件协议和目录创建

- 实现 `aiw vws new`。
- 支持 CLI 参数选择 repo。
- 创建 VWS Git repo。
- 创建每个 repo 的 dev worktree。
- 生成 `.vws` 文件结构。
- 注入 VWS skills 模板。
- 不做自动 agent 调度。

### Phase 2：状态和 review

- 实现 `aiw vws status`。
- 实现 `aiw vws diff` 和 `aiw vws snapshot`。
- 生成 `.vws/reviews/*`。
- 检测 dev worktree 是否被直接修改。

### Phase 3：Paseo 调度桥

- 检测 `paseo` CLI 和 daemon 状态。
- 实现 `aiw vws dispatch --backend paseo`。
- 使用 `paseo run --detach --cwd ... --label ...` 创建 worker。
- 写入 `.vws/state/dispatch.yaml`。
- 支持 Paseo leader agent 调用 AIW VWS tools。

### Phase 4：任务合入和 close

- 实现 `aiw vws task done`。
- feature worktree 合入 repo dev。
- 实现 `aiw vws close --cloud-merge`。
- 生成云端合并交付材料。
- 清理 VWS 本地 worktrees。

## 关键约束

- VWS dev worktree 不直接开发。
- 子任务开发只发生在 task feature worktree。
- VWS close 不直接合入 master。
- 业务 repo 不默认写入 AIW 私有文件。
- Leader agent 的状态必须文件化。
- Worker agent 默认是后台 subagent。
- 调度后端可替换，Paseo 是首选 control plane，不是唯一执行后端。
- AIW 不做 agent provider，不绑定 Codex、Claude、OpenCode 或其他 agent。

## 待决问题

- VWS 根目录默认放在哪里：`~/.aiw/vws`、`~/.paseo/workspaces` 还是用户指定目录。
- VWS dev branch 命名是否统一为 `vws/<name>/dev`。
- Task feature branch 是否统一为 `vws/<name>/task/<task-id>`。
- Paseo worker 是否默认可见，还是默认 detached。
- `aiw vws new` 是否默认启动 Paseo leader。
- `vws close --cloud-merge` 的云端合并协议如何表达：MR 列表、CI gate、reviewer、发布顺序。
- 是否需要 `repo-lock.yaml` 记录每个 repo close 时的 commit SHA。
- 是否保留 submodule/gitlink 风格的 archive 锚点。
