# aiw

`aiw` 是一个个人 AI 编程工作流 CLI。它把一组锋利的终端工具串成稳定路径：开始任务、打开适合 AI 协作的 workspace、审查改动、生成提交、清理 worktree。

默认文档语言是英文。英文版见 [README.md](./README.md)。

## aiw 是什么

AIW 是一个很薄的编排层。它不替代你已经在用的工具，而是决定下一步应该调用什么，检查依赖是否齐全，然后把具体能力交还给对应工具。

边界如下：

- `aiw` 负责工作流决策、配置读取、依赖门禁、prompt、命令路由和 cmux layout 生成。
- Worktrunk 负责 worktree 生命周期。
- cmux 负责 workspace 和 pane。
- lazygit 负责 Git TUI 操作。
- delta 负责 diff 渲染。
- agent CLI 负责模型交互。
- yazi、nvim、rg、fd、fzf、bat、eza 保持各自原生职责。

因此，AIW 的目标不是变成终端模拟器、Git 客户端、编辑器、diff 查看器、守护进程、任务数据库或 agent 管理器，而是把个人工作流固定下来。

## 快速上手

优先从已发布的 npm 包启动。用户开始使用 AIW 不需要先 clone 这个仓库：

```bash
npx @chlrc/aiw init
aiw doctor
aiw cmux-new --agent codex
```

第一次可以直接使用 `npx @chlrc/aiw ...`。后续高频使用时，建议安装包或拉到本地 checkout，方便启动、调试和个性化配置。

从本地 checkout 运行，主要用于开发 AIW 自身：

```bash
node bin/aiw --help
node bin/aiw doctor
node bin/aiw cmux-new --agent codex
```

日常工作流通常是：

```bash
# 1. 检查本机工具链是否就绪。
aiw doctor

# 2. 创建或切换 Worktrunk worktree，并打开 AIW cmux layout。
aiw cmux-new --agent codex

# 3. 在三 pane workspace 里工作。
# Files: yazi
# Agent: codex / claude / opencode / gemini / aider
# Git: lazygit + AIW overlay，并用 delta 渲染 diff

# 4. 审查并 stage 改动。
aiw git

# 5. 基于 staged changes 生成 commit message 并提交。
aiw commit

# 6. feature worktree 完成后合并并清理。
aiw done dev
```

开发 AIW 自身时，优先运行 checkout 内 CLI：

```bash
node bin/aiw doctor
node bin/aiw doctor --gate cmux-new --agent codex
node bin/aiw layout --agent codex --dry-run
node bin/aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --agent codex --dry-run
```

## 安装模型

AIW 是 Node.js ESM CLI，要求 Node.js 18 或更新版本。可执行入口是 [bin/aiw](./bin/aiw)，package metadata 把它暴露为 `aiw` binary。

本地开发时，直接运行 `node bin/aiw ...` 即可。需要全局命令时，如果 checkout 内存在安装脚本，可运行：

```bash
npm run install:global
```

AIW 按以下顺序读取配置：

1. 设置了 `$AIW_CONFIG_DIR` 时使用它。
2. 存在 `~/.config/aiw` 时使用它。
3. 否则使用仓库内默认 [config/](./config) 目录。

这样仓库可以提供开箱默认值，同时允许个人配置留在业务仓库之外。

## Agent Skills

AIW 现在按 npm `skills` CLI 可消费的 multi-skill 目录组织 agent skills：

- [skills/aiw-init](./skills/aiw-init/SKILL.md)：帮助初始化和排查 AIW 环境。
- [skills/aiw-reference](./skills/aiw-reference/SKILL.md)：帮助 agent 直接执行 AIW workspace、Git、commit、done、remove 和 GC 流程。

当前 `skills add` 流程消费 Git/GitHub 源，因此用户不需要 clone repo，也可以直接从这个仓库安装 AIW skills：

```bash
npx --yes skills add KiritoKing/aiw-cli --list -y
npx --yes skills add KiritoKing/aiw-cli --skill aiw-init -y
npx --yes skills add KiritoKing/aiw-cli --skill aiw-reference -y
```

维护者发布前可以用本地 checkout 验证 discoverability：

```bash
npx --yes skills add . --list -y
```

`aiw init` 暂不安装这些 skills；它只初始化 AIW config 和 cmux 集成。

## 初始化

在 macOS 或 Linux 上，可以用 `npx @chlrc/aiw init` 为一台机器初始化 AIW。

初始化流程假设 Node.js 和 `npx` 已经可用。它会检查必要环境变量、POSIX shell 平台，以及默认工作流所需的阻塞依赖；如果缺少阻塞依赖，会在写文件前停止并给出安装建议。`fd`、`eza`、非默认 agent CLI 这类可选工具只报告，不阻塞。

依赖门禁通过后，init 会创建缺失的目录和配置文件：

```bash
npx @chlrc/aiw init
npx @chlrc/aiw init --cmux-scope home
npx @chlrc/aiw init --cmux-scope code --code-root ~/Code --worktrees-root ~/worktrees
npx @chlrc/aiw init --sessions-root ~/Documents/aiw
npx @chlrc/aiw init --cmux-scope none --dry-run
```

- AIW 配置默认写入 `~/.config/aiw`；设置了 `$AIW_CONFIG_DIR` 时使用它。
- `--code-root` 和 `--worktrees-root` 控制写入 `aiw.toml` 的路径，默认是 `~/Code` 和 `~/worktrees`。
- `--sessions-root` 控制写入 `aiw.toml` 的 scratch 会话根目录，默认是 `~/Documents/aiw`。
- cmux 注册默认写入 `~/.config/cmux/cmux.json`；`--cmux-scope code` 写入 `<code-root>/.cmux/cmux.json`；`--cmux-scope none` 跳过 cmux。
- 交互式终端中，init 会询问 cmux 注册位置；`--yes` 使用默认值且不提示。
- 已存在的 AIW 配置默认保留；使用 `--force` 覆盖前会先创建备份。
- 已存在的 cmux 配置会合并，不会整体替换；AIW action 会被加入，已有非 AIW plus-button action 会通过 context menu 保留。
- Skills 暂不由 init 初始化。

## 技术栈

AIW 有意保持低依赖：

- Runtime: Node.js ESM，Node >= 18。
- Config: 少量 TOML 文件，由 AIW 自己解析。
- Process execution: 通过本地 helper 调用 Node child process。
- Workspace orchestration: Worktrunk (`wt`) 和 cmux。
- Git UI: lazygit；仅 `aiw git` 会加载 AIW 专用 overlay。
- Diff UI: 默认通过 lazygit overlay 使用 delta；独立 `aiw diff` 命令仍优先 `cmux-git-diff`，否则 `git diff | delta`。
- Picker 和搜索: fzf、rg、fd、bat。
- 文件和编辑界面: yazi、nvim。
- Agent: 在 [config/agents.toml](./config/agents.toml) 中声明命令适配器。

当前还没有完整自动化测试套件。代码改动后必须运行：

```bash
npm run check
```

风险较高的 CLI 行为建议在 `/private/tmp` 下创建临时 Git repo，并尽量用非交互命令验证。

## 心智模型

可以把 AIW 理解成三层：

1. cmux 是入口。plus button、command palette，或者 cmux 里的 terminal action，是新任务开始的地方。
2. `aiw` 是编排入口。它负责解析 repo、branch/worktree、agent、依赖门禁和 layout，然后调用正确的底层命令。
3. 成熟 CLI 工具负责真正执行：Worktrunk 管 worktree，yazi 管文件，nvim/LazyVim 管编辑，lazygit 管审查和 stage，delta 管 diff，rg/fd/fzf/bat/eza 管搜索和查看，zoxide 融入外层 shell 导航，agent CLI 负责模型交互。

AIW 最初的目标很实际：在 Codex App 之外复刻它里面有价值的 workspace 动线，但不把 workflow 搬进一个封闭 App。目标不只是“屏幕上有几个 pane”，而是一条连续动作：打开 cmux，启动 AIW，创建或切换 worktree，进入标准 workspace，看文件，和 agent 交流，stage hunk，在 lazygit 里看 delta 渲染的 diff，commit，merge，清理。

关键差异是控制权。成熟 App 的体验可以很完整，但它通常也固定了工作流形状。在终端工具栈里，你可以选择文件管理器、编辑器配置、Git UI、diff renderer、fuzzy finder、目录跳转工具、shell 行为、快捷键和 agent CLI。shell 渲染也是体验的一部分：当命令输出是开发过程的核心时，原生终端工具比嵌入式 shell 界面更可信、更可调，也更容易拼装。

所以 AIW 保留 Codex-style workspace 的模式，但把它做成 cmux-first、terminal-native 和 agent-neutral。同一套动线应该可以运行在 Codex、Claude、opencode、Gemini、aider，或者任何未来遵守同一配置契约的 agent adapter 上。如果你对自定义性要求很强，希望把开发控制权还给自己的终端工具栈，AIW 就是这套实验。

开始工作时，AIW 要回答四个问题：

1. 当前要处理哪个 repository？
2. 这项工作应该放在哪个 worktree 或 branch？
3. 应该在代码旁边打开哪个 agent？
4. 工作时默认可见哪些 pane？

答案确定后，AIW 就停止做领域决策，把执行交给底层工具：

```text
aiw cmux-new
  -> dependency gate
  -> repository and branch selection
  -> Worktrunk creates or switches the worktree
  -> aiw layout builds the cmux workspace
  -> cmux opens Files / Agent / Git panes
```

清理流程也是同一套思想：

```text
aiw workspace gc
  -> read Worktrunk and Git state
  -> mark dirty, merged, stale, and cmux-open signals
  -> remove only clean + merged worktrees when explicitly confirmed
```

AIW 不试图隐藏底层工具。它做的是缩短常见路径，并把危险路径显式化。

## 设计逻辑

### cmux 是入口，AIW 是编排器

预期入口是 cmux，而不是一个独立的 AIW dashboard。cmux 已经负责窗口、workspace、pane，以及用户真实看到的开发界面。AIW 站在这个入口背后，作为决定打开什么、怎么串起来的命令。

所以主动线是：

```text
cmux entry
  -> aiw cmux-new
  -> dependency gate
  -> Worktrunk worktree create/switch
  -> aiw layout
  -> cmux workspace with Files / Agent / Git
```

价值在于层与层之间的交接很流畅。cmux 继续作为开始工作和承载视图的地方；AIW 作为 workflow brain；底层 CLI 继续保持锋利和原生。

### 复刻 App 动线，而不是复刻 App 锁定

AIW 借用了 Codex App-style 界面里最有价值的部分：把代码 workspace、agent、Git 审查和 diff loop 放在一起，并让隔离任务的启动变得自然。它不借用锁定。整个 workflow 表达为命令、配置文件和 cmux layout，因此每一层都可以替换。

这也是为什么 agent 支持是 adapter-based。agent 不硬编码进产品里。它只是一个命令，加上用于交互式 pane 的可选 args，以及用于一次性生成 commit message 的 `commit_args`。

### 不把个人工作流写进业务仓库

AIW 默认不向业务仓库写入 `.aiw`、`.worktrunk` 或 `.cmux` 文件。AIW 自己拥有的运行时 metadata，例如创建 worktree 时记录的 `feature -> target`，写在 Git common dir 下，而不是业务 working tree 里。

### 让终端继续作为控制平面

AIW 假设用户应该能够替换工具、调整渲染、保留原生 shell 行为，并继续使用已有肌肉记忆。它组合 rg、zoxide、yazi、nvim/LazyVim、lazygit、delta、fzf、Worktrunk、cmux 和 agent CLI 这类成熟工具，而不是把它们压平成一个新的封闭 UI。

这也是为什么 shell 和 diff 输出继续留在 terminal-native 工具里。终端不是 AIW 的 fallback surface，而是主要控制平面。

### 有副作用前先过门禁

会创建 worktree 或打开 cmux layout 的命令会先跑 dependency gate。如果外部工具缺失，AIW 会在创建新 workspace 前停止。

常用检查：

```bash
aiw doctor
aiw doctor --gate git
aiw doctor --gate cmux-new --agent codex
aiw doctor --gate scratch --agent codex
aiw doctor --gate commit --agent codex
aiw doctor --json
```

### 人类交互时可选择，脚本调用时要显式

在 TTY 中，缺少必要参数时会尽量打开可搜索 picker。非交互调用中，应显式传入参数，或保留底层工具默认行为。

例如：

```bash
aiw cmux-new
aiw cmux-new --pick-repo
aiw workspace open
aiw workspace open feat/foo --agent codex
```

### 保留原生工具职责

`aiw git` 打开 lazygit，不重写 stage、hunk 选择或提交前审查。

`aiw files` 打开 yazi，不变成文件管理器。

`aiw diff` 渲染当前 Git diff，不实现自定义 diff engine。

## 核心命令

```bash
npx @chlrc/aiw init
aiw doctor
aiw cmux-new --agent codex
aiw new --agent codex
aiw cmux-new --pick-repo --agent codex
aiw cmux-new --local --agent codex
aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --agent codex --dry-run
aiw layout --agent codex --dry-run
aiw cmux scratch --agent codex --dry-run

aiw workspace list
aiw workspace list --json
aiw workspace open
aiw workspace open feat/foo --agent codex
aiw workspace done dev
aiw workspace remove feat/foo
aiw workspace gc
aiw workspace gc --dry-run
aiw workspace gc --apply

aiw git
aiw diff
aiw diff --watch
aiw diff --staged
aiw commit
aiw commit --agent codex
aiw commit --prompt "Use scope aiw"
aiw commit --prompt-file ~/commit-style.md

aiw files
aiw edit src/file.ts:10
aiw grep keyword
aiw pick
aiw tree 3
```

日常使用保留了 workspace 短 alias：

```bash
aiw ws list
aiw ws ls
aiw list
aiw ls
aiw als
aiw open feat/foo
aiw switch feat/foo
aiw done
aiw remove feat/foo
aiw gc
aiw clean
```

## Worktree 和 cmux 工作流

`aiw cmux-new` 是新任务的主入口。`aiw new` 和 `aiw cmux new` 是同一个命令的 alias。

常见形式：

```bash
aiw cmux-new
aiw new
aiw cmux-new --agent codex
aiw new --agent codex
aiw cmux-new --pick-repo --agent codex
aiw cmux-new --repo ~/Code/my-repo --agent codex
aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --agent codex
aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --base main --agent codex
aiw cmux-new --local --agent codex
```

行为：

- 能识别当前 Git repo 时，默认使用当前 repo。
- `--pick-repo` 可以选择 `paths.code_root` 下的其他 repo。
- TTY 中没有传 branch 时，AIW 会让你选择从当前 `HEAD` 创建新分支、从已有 branch 创建新分支、打开当前 checkout、或选择已有分支。
- 创建新分支时运行 `wt switch --create <branch> --base @ -x "aiw layout --agent <agent>"`。
- 从已有 branch 创建新分支时运行 `wt switch --create <branch> --base <base> -x "aiw layout --agent <agent>"`；非交互使用 `--branch <new-branch> --base <branch>` 或 `--from <branch>`。
- 选择已有分支时运行 `wt switch <branch> -x "aiw layout --agent <agent>"`。
- `--local` 在当前 checkout 打开标准 layout，不创建 Worktrunk worktree。
- `--dry-run` 打印将要运行的命令。

标准 cmux layout：

```text
+----------------------+----------------------+
| Files                | Agent                |
| aiw files            | codex/claude/etc.    |
+----------------------+----------------------+
| Git                                         |
| aiw git                                     |
+--------------------------------------------+
```

`aiw layout --print-json` 可以只打印 cmux layout JSON，不打开 cmux。

## Scratch 会话

当你想打开一个不绑定 Git repo 或 Worktrunk worktree 的 AI-ready cmux 会话时，使用 `aiw cmux scratch`。`aiw scratch` 是短别名。

```bash
aiw scratch
aiw cmux scratch
aiw scratch --agent codex
aiw scratch "api-notes" --agent codex
aiw cmux scratch --agent codex
aiw session --root /private/tmp/aiw-sessions --id smoke --agent codex --dry-run
```

行为：

- 会话创建在 `paths.sessions` 下，默认是 `~/Documents/aiw`。
- 目录结构是 `YYYY-MM-DD/<session-id>`。
- 不传显式 ID 时，AIW 会生成 `HHMMSS-<uuid8>`。
- Scratch layout 只打开 Files 和 Agent，不打开 Git，也不调用 Worktrunk。
- `--root <path>` 可以临时覆盖 `paths.sessions`。
- `--dry-run` 只打印目录和 cmux 命令，不创建任何内容。

## Workspace 管理

`aiw workspace ...` 是 Worktrunk worktree 的可见性和生命周期层。

### 列表

```bash
aiw workspace list
aiw workspace list --json
aiw workspace list --stale-seconds 604800
aiw workspace states
```

表格会合并 Worktrunk、Git 和 cmux 信号：

- branch 和 path
- dirty 或 clean 状态
- Worktrunk integration state
- AIW 创建 worktree 时记录的 merge target
- 没有记录 target 时推断出的 merged target
- cmux 是否已打开
- age
- GC signal

这些状态标记刻意保持保守。dirty worktree 永远不会成为自动清理候选。

### 打开

```bash
aiw workspace open
aiw workspace open feat/foo --agent codex
aiw workspace open /path/to/worktree --agent codex
aiw open feat/foo
```

TTY 中不传 target 时，`workspace open` 会打开可搜索 picker，列出现有 worktree 和本地 branch。branch target 通过 Worktrunk 打开；path target 则直接在对应 Git root 打开 AIW cmux layout。

### 收尾

```bash
aiw workspace done
aiw workspace done dev
aiw workspace done dev --agent codex
aiw workspace done dev --retries 3
aiw workspace done dev --agent codex --retries 3
aiw workspace done dev --no-close-cmux
aiw done dev
```

`done` 只能在 feature worktree 内执行。主 workspace 会被拒绝；当前 worktree dirty 时也会被拒绝。如果目标分支正被另一个 dirty worktree checkout，也会在进入 merge 前拒绝。

通过检查后，AIW 把 merge 和 cleanup 交给 Worktrunk：

```bash
wt merge <target>
```

Worktrunk 默认会 squash。如果 Worktrunk 没有配置 `[commit.generation] command`，也没有显式设置 `WORKTRUNK_COMMIT__GENERATION__COMMAND`，AIW 会为本次 merge 注入自己的 commit message bridge：

```bash
WORKTRUNK_COMMIT__GENERATION__COMMAND="<aiw> commit-message --agent <agent>" wt merge <target>
```

这样可以避免 Worktrunk 的普通 fallback squash subject 触发业务仓库的 Conventional Commit hook 失败。传 `--agent <name>` 可以指定本次 squash message 使用哪个 AIW commit agent；已有 Worktrunk commit-generation 配置和显式环境变量会被保留，不会被 AIW 覆盖。

如果 AIW 在创建 worktree 时记录过 target，裸 `aiw done` 可以使用该 target。TTY 中缺少 target 时会打开目标分支 picker。`done` 会按 `commit.retries` 重试失败的 `wt merge`，默认 3 次；每次失败后会恢复 source worktree、target branch 和 Worktrunk backup ref，避免把中间脏状态留在仓库里。成功 merge 后，AIW 默认关闭匹配的 cmux workspace；传 `--no-close-cmux` 可保留。

### 删除和 GC

```bash
aiw workspace remove feat/foo
aiw workspace remove feat/foo --force
aiw workspace gc
aiw workspace gc --dry-run
aiw workspace gc --apply
aiw workspace gc --yes
aiw workspace gc --json
```

`remove` 会在调用 `wt remove` 前做 dirty check。只有明确要把风险交给 Worktrunk 时才使用 `--force`。

`gc` 拆分三类信号：

- `dirty`: 阻止自动清理。
- `merged`: branch 已 integrated、same commit、empty，或确认已包含在目标分支。
- `stale`: 最近一次 commit 时间超过 `workspace.stale_seconds`，默认七天。

只有 clean + merged 的 worktree 会进入可删除候选。stale 但 dirty 或未合并的 worktree 只会产生 warning。

### Workspace hooks

Workspace hooks 可以在 AIW 初始化 cmux workspace 前，或删除 worktree 前运行 shell 命令。全局 hooks 配在 `~/.config/aiw/aiw.toml`；项目 hooks 配在 Git repo 根目录下显式存在的 `.aiw.toml`。AIW 只读取已存在的项目配置文件，不会主动创建业务仓库里的个人 workflow 文件。

```toml
[workspace.hooks]
pre_init = ["echo preparing $AIW_WORKSPACE_PATH"]
pre_remove = ["echo removing $AIW_WORKSPACE_TARGET"]

[workspace.hooks.projects.my-repo]
pre_init = ["echo preparing only $AIW_PROJECT_NAME"]
pre_remove = ["echo cleaning only $AIW_PROJECT_PATH"]

[workspace.hooks.projects.my-alias]
match = "repo.name.with.dots"
path = "~/Code/repo.name.with.dots"
pre_init = ["echo project matched by name or path"]
```

执行顺序是：全局 hooks、匹配的全局项目 hooks、项目本地 hooks。`[workspace.hooks.projects.<name>]` 默认用 `<name>` 匹配当前 worktree 名和 Git common-dir 项目名；如果表名只是别名，或需要按路径匹配，可以使用 `match`、`path`、`paths`、`repo`、`repos`。支持 `*` 通配符。

命令会在目标 workspace path 下通过 `sh -lc` 顺序执行；任一 hook 失败都会中断当前动作。`pre_init` 在 `aiw layout` 调用 `cmux new-workspace` 前执行。`pre_remove` 在 `aiw workspace done`、`aiw workspace remove` 和 `aiw workspace gc --apply` 通过 Worktrunk 删除路径前执行。

Hook 命令会收到这些环境变量：`AIW_HOOK_EVENT`、`AIW_HOOK_SOURCE`、`AIW_HOOK_CONFIG`、`AIW_HOOK_RULE`、`AIW_REPO`、`AIW_PROJECT_NAME`、`AIW_PROJECT_PATH`、`AIW_WORKSPACE_PATH`、`AIW_WORKSPACE_BRANCH`、`AIW_WORKSPACE_TARGET`、`AIW_AGENT`。

## Git、Diff 和 AI Commit

### lazygit Overlay

`aiw git` 用 AIW overlay 打开 lazygit：

```bash
lazygit --use-config-file <aiw-config-dir>/lazygit-delta.yml
```

这个 overlay：

- 使用 delta 展示 diff
- 增加全局 `Ctrl-A` 自定义命令
- 询问可选 AI commit instruction
- 运行 `aiw commit --prompt ...`

直接运行 `lazygit` 不受影响。

### Diff

```bash
aiw diff
aiw diff --watch
aiw diff --staged
aiw diff --all
```

对 unstaged diff，AIW 优先使用 `cmux-git-diff`。缺失时，使用 Git diff 输出并通过 delta 渲染。`--watch` 每两秒刷新。

### AI Commit

`aiw commit` 只处理 staged changes，不会默默帮你 stage 文件。

```bash
aiw git
aiw commit
aiw commit --agent codex
aiw commit --prompt "Use scope aiw"
aiw commit --prompt-file ~/commit-style.md
aiw commit --print-prompt
aiw commit --dry-run
aiw commit-message --agent codex < prompt.txt
```

流程：

1. 读取 `git diff --cached`、`git diff --cached --stat` 和 `git status --short`。
2. 基于 [config/commit-prompt.md](./config/commit-prompt.md) 构造 prompt。
3. 追加 `--prompt` 和 `--prompt-file` 内容。
4. 按 [config/agents.toml](./config/agents.toml) 的 `commit_args` 调用选定 agent。
5. 清理 agent 输出，得到 commit message。
6. 运行 `git commit -F -`。
7. 如果 commit hook 失败，把 hook 输出注入下一轮 prompt，并按 `commit.retries` 重试。

`aiw commit-message` 是 `aiw done` 桥接 Worktrunk squash commit generation 时使用的 stdin-to-stdout message generator。它不会创建 Git commit。

Agent adapter 契约是 stdin/stdout 风格：

- stdin 或 `{{prompt}}`: 完整 commit prompt 和 staged diff
- stdout: 只输出 commit message
- stderr: 日志和诊断信息

## 配置

默认配置文件在 [config/](./config)：

- [config/aiw.toml](./config/aiw.toml): defaults、paths、behavior、commit、Git 和 workspace 设置
- [config/agents.toml](./config/agents.toml): agent command adapters
- [config/commit-prompt.md](./config/commit-prompt.md): 基础 commit prompt
- [config/lazygit-delta.yml](./config/lazygit-delta.yml): 仅 `aiw git` 使用的 lazygit overlay

关键默认值：

```toml
[defaults]
agent = "codex"
editor = "nvim"
files = "yazi"
git = "lazygit"

[paths]
code_root = "~/Code"
worktrees = "~/worktrees"
sessions = "~/Documents/aiw"

[commit]
agent = "codex"
retries = 3
max_diff_chars = 120000

[workspace]
stale_seconds = 604800

[workspace.hooks]
pre_init = []
pre_remove = []
```

Agent 配置示例：

```toml
[agents.codex]
cmd = "codex"
args = []
commit_args = ["exec", "--skip-git-repo-check", "--sandbox", "read-only", "--color", "never", "-"]
```

`args` 用于交互式 agent pane，`commit_args` 用于一次性 commit message 生成。

## 依赖门禁

`aiw doctor` 会展示工具可用性并评估 gate。默认 gate 是 `p0`。

常用 gate：

```bash
aiw doctor --gate git
aiw doctor --gate workspace
aiw doctor --gate layout --agent codex
aiw doctor --gate scratch --agent codex
aiw doctor --gate cmux-new --agent codex
aiw doctor --gate init --agent codex
aiw doctor --gate commit --agent codex
```

Gate 行为：

- `cmux-new` 需要 Git、Worktrunk、cmux、yazi、lazygit、nvim、选定 agent；配置 lazygit overlay 时还需要 delta。
- `layout` 需要 layout 相关工具、选定 agent，以及同一组 lazygit overlay 依赖，但不需要 Worktrunk。
- `scratch` 需要 cmux、yazi、nvim 和选定 agent，不需要 Git、Worktrunk、lazygit 或 delta。
- `init` 在写配置前检查默认工作流初始化所需工具。
- `workspace` 需要 Git 和 Worktrunk。
- `git` 在配置 lazygit overlay 时需要 lazygit 和 delta。
- `commit` 需要 Git 和选定 commit agent。

缺失的可选工具只在对应命令或 gate 需要时才阻塞。例如缺少 `fd` 会影响 `aiw pick`，不会影响 `aiw commit`。

## 故障排查

### `dependency gate ... failed`

运行对应 doctor 命令：

```bash
aiw doctor --gate cmux-new --agent codex
```

安装缺失工具，或修改 AIW 配置使用其他命令。

### `no staged changes`

`aiw commit` 只读取 staged changes。请先在 lazygit 或 Git CLI 中 stage：

```bash
aiw git
git add <path>
aiw commit
```

### `not inside a Git repository`

大多数 AIW 工作流都基于 Git root。请在 Git repository 中运行，传 `--repo`，或在支持的命令里使用 `--pick-repo`。

### `aiw workspace done must be run from a feature worktree`

`done` 是 feature worktree 收尾命令。请在 feature worktree 内执行，而不是在主 checkout 里执行。

### Dirty Worktree 阻塞 remove、done 或 GC

AIW 会保护未提交改动。先审查它们：

```bash
aiw git
```

只有明确要删除或交给 Worktrunk 决策时，才在 `workspace remove` 上使用 `--force`。

### Shell 输出 `fnm_multishells ... Operation not permitted`

本地环境中可能在命令输出前出现这类信息。如果实际 AIW 命令成功，可以把它视为 shell startup 噪音，而不是 AIW 失败。

## 开发指南

非平凡改动前先读：

- [README.md](./README.md): 给人类看的使用说明书
- [AGENTS.md](./AGENTS.md): 给 AI agent 看的开发规范
- [docs/2026-05-29-design.md](./docs/2026-05-29-design.md): 原始设计记录
- [docs/2026-06-01-workflow-handoff.md](./docs/2026-06-01-workflow-handoff.md): workflow 历史记录
- [docs/2026-06-01-workspace-management-plan.md](./docs/2026-06-01-workspace-management-plan.md): workspace 管理记录
- [handoff.md](./handoff.md): 当前活动 handoff

代码改动后运行：

```bash
npm run check
```

额外定向检查：

```bash
node bin/aiw doctor --gate git
ruby -ryaml -e 'YAML.load_file("config/lazygit-delta.yml")'
node bin/aiw doctor --gate commit --agent codex
node bin/aiw doctor --gate cmux-new --agent codex
node bin/aiw layout --agent codex --dry-run
node bin/aiw init --cmux-scope none --dry-run --no-reload
```

修改 `src/commit.mjs` 时，用有 staged changes 的临时 Git repo 验证。修改 worktree 行为时，先用 dry-run，再用 `/private/tmp` 下的一次性 repo 验证。

## 项目结构

```text
bin/aiw                    executable entrypoint
src/cli.mjs                top-level command dispatch
src/config.mjs             config loading and agent resolution
src/deps.mjs               dependency gates and doctor output
src/git.mjs                Git repo, repo picker, branch selection helpers
src/layout.mjs             cmux layout generation
src/hooks.mjs              workspace hook loading and execution
src/init.mjs               first-time machine bootstrap
src/workspace.mjs          workspace list/open/done/remove/gc
src/commit.mjs             AI commit workflow
src/agent.mjs              agent invocation and output cleanup
src/prompt.mjs             TTY input and fzf picker helpers
src/run.mjs                process execution helpers
config/aiw.toml            default workflow config
config/agents.toml         agent adapters
config/commit-prompt.md    base AI commit prompt
config/lazygit-delta.yml   lazygit overlay used by aiw git
docs/                      design notes and dated handoff records
handoff.md                 current activity handoff
```

## 当前状态

AIW 当前聚焦四条稳定路径：

- 创建或打开 worktree，并进入适合 AI 协作的 cmux layout
- 安全地查看和清理 workspace
- 通过 lazygit 和 delta 审查改动
- 通过 agent CLI 基于 staged diff 生成 commit message

后续演进应继续保持这个边界：AIW 负责协调，专业工具负责专业能力。
