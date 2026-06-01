# AIW Workspace / Worktree 管理计划

日期：2026-06-01

## 当前结论

`aiw` 已经把 workspace / worktree 管理收敛到 `aiw workspace ...` 下；顶层短命令保留为 alias，减少日常输入。

项目规范：TTY 下缺少必要参数时默认提供可搜索 TUI/picker；非交互场景才报错或保持底层工具默认行为。

已完成能力：

- `aiw cmux-new`：默认从当前 repo 的当前 HEAD 创建新分支和 Worktrunk worktree，然后运行 `aiw layout` 打开 cmux 四分屏。
- `aiw cmux-new --pick-repo`：手动选择 `~/Code` 下的 repo。
- `aiw cmux-new --local`：显式在当前 checkout 打开 AIW 四分屏，不创建 worktree。
- `aiw workspace list` / `status`：列出 Worktrunk worktree、dirty 状态、集成状态、已合入目标、年龄、GC 信号、cmux workspace 是否已打开、路径，并用颜色突出 dirty/clean/open/removable/stale。
- `aiw workspace list --json`：输出结构化 workspace 信息。
- `aiw workspace open [target]` / `switch [target]`：TTY 下无 target 时打开 AIW 可搜索 picker，列出已有 worktree 和本地 branch；指定 branch/path 时，通过 Worktrunk 切换并打开标准 AIW 四分屏。
- `aiw workspace done [target]`：仅允许在 feature worktree 内执行；主工作区会直接报错，不进入分支选择器。通过检查后调用 `wt merge [target]`；TTY 下没有 target 时进入本地分支选择器，并优先用创建 worktree 时记录的 target 作为默认值。成功后默认关闭对应 cmux workspace，可用 `--no-close-cmux` 保留。
- `aiw workspace remove` / `rm`：按目标 worktree 做 dirty check 后调用 `wt remove`。
- `aiw workspace gc` / `clean`：输出可删除候选和 stale 警告；TTY 下确认 `y` 后可直接删除安全候选，也支持 `--apply` / `--yes` 非交互清理。stale 阈值用 Unix 秒配置，默认 `604800`。
- 顶层 alias：
  - `aiw ws ...` -> `aiw workspace ...`
  - `aiw list` -> `aiw workspace list`
  - `aiw open <target>` -> `aiw workspace open <target>`
  - `aiw switch <target>` -> `aiw workspace open <target>`
  - `aiw done` -> `aiw workspace done`
  - `aiw remove` -> `aiw workspace remove`
  - `aiw gc` / `aiw clean` -> `aiw workspace gc`
- `<code-root>/.cmux/cmux.json`：plus-button 默认接 `AIW New Worktree`，右键菜单保留手动选 repo 和本地 workspace 入口。

## 什么叫 workspace / worktree 管理

目标不是把 `aiw` 做成 Git 客户端、任务数据库或 agent 管理器，而是把已有工具串成稳定、可恢复、少踩坑的工作流。

应该覆盖这些能力：

1. 可见性：一眼看清当前有哪些 worktree、对应分支、路径、dirty 状态、是否有上游、是否已经打开 cmux workspace。
2. 打开和切换：从列表里选择一个已有 worktree，切过去并打开标准 AIW 四分屏，而不是只执行裸 `wt switch`。
3. 新建：从当前分支创建新 worktree，这是当前 `aiw cmux-new` 已覆盖的核心路径。
4. 本地 checkout：作为显式附加入口保留，避免误把当前业务 checkout 当新 worktree 使用。
5. 收尾：检查 dirty 状态，合并当前 feature 到目标分支，例如 `aiw done dev`，再删除 worktree；必要时给出明确的阻塞原因和下一步命令。
6. 目标记录：AIW 创建 worktree 时记录 `feature -> target` 到 Git common dir 下的本地 metadata，不写入业务工作树；`done/list/gc` 优先使用该 target 判断。
7. 清理：把 dirty、merged/integrated、stale 作为独立信号。clean+merged 可作为可删除候选；dirty+stale 只作为 stale 警告，不自动清理。
8. cmux 关联：workspace 名称、cwd、分支、状态尽量和 worktree 对齐；切换/打开时使用同一套 AIW layout。

## 建议实现顺序

### Phase 1：只读盘点（已完成）

已新增：

```bash
aiw workspace list
aiw workspace status
aiw workspace list --json
```

数据来源：

- `wt list --format json`：Worktrunk 视角，包含集成状态和 dirty 摘要。
- `git worktree list --porcelain`：Worktrunk JSON 不可用时的 fallback。
- `git status --short`：fallback dirty 状态。
- `git merge-base --is-ancestor <branch> <target>`：补充检测 feature 是否已合入某个本地非 worktree 分支，例如 `dev`。
- `cmux list-workspaces --json`：标记哪些 workspace 已在 cmux 里打开。

输出重点：

- repo
- branch
- worktree path
- dirty / clean
- merged target
- age / GC signal
- cmux workspace 是否已打开

后置：

- upstream / ahead-behind

### Phase 2：打开已有 worktree（已完成）

已新增：

```bash
aiw workspace open
aiw workspace open <branch>
aiw workspace switch <branch>
aiw open <branch>
aiw switch <branch>
```

期望行为：

- TTY 下无 target 时使用 AIW 可搜索 picker，默认包含已有 worktree 和本地 branch；指定 target 时按 branch/path 打开。非交互场景回落到 Worktrunk picker 行为，脚本里建议显式传 target。
- 有 branch 时调用 Worktrunk 切到对应 worktree；path/detached worktree 直接在该 path 上打开 layout。
- 自动运行 `aiw layout --agent <agent>` 打开标准四分屏。

这一步已经补齐原来 `aiw switch` 只是 `wt switch` passthrough 的缺口。

### Phase 3：安全收尾和清理

已完成基础 alias 和 GC dry-run：

```bash
aiw workspace done
aiw workspace done dev
aiw workspace remove
aiw workspace gc
aiw workspace clean
aiw workspace gc --apply
aiw workspace gc --yes
aiw workspace gc --stale-seconds 604800
```

已完成：

- dirty 时明确阻塞，不自动丢弃用户改动；`remove <branch>` 优先检查目标 worktree，没有目标时检查当前 worktree。
- clean 且可合并时走 `wt merge <target>`；`aiw done dev` 表示把当前 feature worktree 合回 `dev`。`done` 只允许在 feature worktree 内执行，主工作区会直接拒绝。TTY 下没有 target 时打开本地分支选择器，默认选创建 worktree 时记录的 target；非交互下如果有记录 target 就直接使用。
- GC 判断拆成三条独立信号：
  - dirty：自动清理阻断条件。
  - merged：有记录 target 时，以 `git merge-base --is-ancestor <feature> <target>` 为准；没有记录 target 的旧 worktree 才 fallback 到 Worktrunk 默认分支状态或本地非 worktree 分支包含关系，并在列表里显示为 inferred。
  - stale：按最后提交时间计算，默认 `604800` 秒；stale 只提供 warning，不单独允许删除。

后置：

- remove 前给出将删除的 path / branch / merge 状态。
- `gc --apply` / `gc --yes` 只删除 clean+merged 的 safe candidates；stale warning 不会被删除。

### Phase 4：cmux 深集成

当前 plus-button 已可用，但会先在当前 pane 开一个 terminal tab 来运行 `aiw cmux-new`。

确认结果：

- cmux `command` action 的语义是“在 terminal 里运行 shell text”，target 只有 `currentTerminal` 和 `newTabInCurrentPane`。
- cmux `workspaceCommand` 可以直接创建一个静态 workspace layout，但它引用的是 `commands[].workspace` 里的静态定义。
- cmux CLI 支持 `cmux new-workspace --cwd ... --layout ...` 直接创建 workspace；AIW 的 `layout` 命令已经在用这个能力。
- 对 AIW 这种“先交互选择/输入分支，再由 Worktrunk 决定 worktree path，再生成 layout”的动态流程，cmux 配置本身不能直接表达完整逻辑。

可选方案：

- 保持当前方式：稳定，逻辑都在 AIW，缺点是有一个启动 tab。
- 做静态 `workspaceCommand` 启动器：cmux 直接创建一个 AIW Launcher workspace，里面的 setup terminal 再跑 AIW；仍然有可见 setup terminal，只是从当前 pane 的 tab 变成独立 workspace。
- 后续如果 cmux 增加后台 action 或参数化 workspace command，再改成真正无中间 terminal 的入口。

当前决策：保持现状，除非启动 tab 真的影响使用。

## 暂缓项

- `aiw pick` / `fd`：当前没有明确应用场景，暂缓。
- AI commit 增强：最终 commit hash 摘要、hook 失败非交互测试等先暂缓。

## 文档维护

- 本文记录 workspace / worktree 管理的当前判断和建议路线。
- `handoff.md` 只保留当前活动上下文和下一步需要知道的事实。
- 后续如果实现 `workspace remove` 删除预览，应同步更新 `README.md` 和 `handoff.md`。
