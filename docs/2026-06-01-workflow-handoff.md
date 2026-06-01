# AIW Workflow Handoff - 2026-06-01

## 背景

`aiw` 是个人 AI 编程工作流的统一入口。当前产品边界是：`aiw` 维护 workflow 主逻辑、配置读取、依赖门禁、prompt 构造和工具编排；具体能力继续交给现有工具实现。

核心分工：

- Worktrunk 负责 worktree 生命周期。
- cmux 负责 workspace 和 pane。
- lazygit 负责 Git TUI、stage/unstage、提交前审查。
- delta 负责 diff 展示。
- agent CLI 负责生成内容，例如 commit message。
- yazi、nvim、rg、fd、fzf、bat、eza 保持原生工具职责。

约束：

- 不默认在业务仓库写入 `.aiw`、`.cmux`、`.worktrunk`。
- 不默认启动开发服务器。
- 不在非 Git 目录里自动 `git init`。
- 依赖外部工具的主流程必须先过 dependency gate。
- 当前不要再直接修改 `<code-root>/.cmux/cmux.json`，之前 cmux 配置接入尝试导致原工作流异常，已回滚。

## 已实现能力

### 入口和配置

- CLI 位于 `bin/aiw`，Node.js ESM 实现。
- 配置默认从 `~/.config/aiw` 读取；不存在时使用仓库内 `config/`。
- 当前配置入口：
  - `config/aiw.toml`
  - `config/agents.toml`
  - `config/commit-prompt.md`
  - `config/lazygit-delta.yml`
- `aiw` 已全局可用，用户可以直接运行 `aiw ...`。

### 依赖门禁

`aiw doctor` 会检查本机工具，并支持按场景 gate：

- `aiw doctor --gate git`
- `aiw doctor --gate cmux-new --agent codex`
- `aiw doctor --gate layout --agent codex`
- `aiw doctor --gate commit --agent codex`

当前关键规则：

- `cmux-new` / `layout` 需要 `git`、`cmux`、`yazi`、`lazygit`、`nvim`、目标 agent，并要求 `cmux-git-diff` 或 `delta` 至少存在一个。
- `git` 需要 `lazygit`；如果启用了 lazygit overlay，还需要 `delta`。
- `commit` 需要 `git` 和目标 agent。

最近验证结果：`git`、`cmux`、`wt`、`yazi`、`nvim`、`lazygit`、`delta`、`rg`、`fzf`、`bat`、`eza`、`codex`、`claude`、`opencode`、`gemini` 存在；`fd`、`aider`、`cmux-git-diff` 缺失。

### Worktree + cmux

核心命令：

```bash
aiw cmux-new
aiw cmux-new --pick-repo
aiw cmux-new --repo ~/Code/my-repo
aiw cmux-new --repo ~/Code/my-repo --branch feat/foo --agent codex
aiw layout --agent codex
```

行为：

- repo 默认使用当前 Git 仓库。
- 没有传 repo 时，交互式列出当前仓库和 `~/Code` 下的 Git 仓库。
- branch 支持交互式选择已有本地/远端分支，也支持创建新分支。
- agent 来自 `config/agents.toml` 枚举。
- 创建新分支时使用 Worktrunk：

```bash
wt switch --create <branch> --base @ -x "aiw layout --agent <agent>"
```

- 选择已有分支时使用：

```bash
wt switch <branch> -x "aiw layout --agent <agent>"
```

### 四分屏 layout

`aiw layout` 生成 cmux 四 pane：

```text
+----------------------+----------------------+
| Files                | Agent                |
| aiw files            | codex/claude/etc.    |
+----------------------+----------------------+
| Git                  | Diff                 |
| aiw git              | aiw diff --watch     |
+----------------------+----------------------+
```

右下角 diff 不是 cmux 内置高级 diff。当前逻辑是：

1. 如果存在 `cmux-git-diff`，用它。
2. 否则用 `git diff | delta`。
3. `--watch` 每 2 秒刷新一次。

### lazygit + delta

`aiw git` 不再是裸 `lazygit` 转发，而是会读取 `config/aiw.toml` 的：

```toml
[git]
lazygit_config = "lazygit-delta.yml"
```

然后启动：

```bash
lazygit --use-config-file <aiw-config-dir>/lazygit-delta.yml
```

`config/lazygit-delta.yml` 当前包含：

- `git.pagers` 使用 `delta --dark --paging=never --line-numbers`。
- `customCommands` 增加 AI commit 快捷键。

这个改动只影响 `aiw git`，不会修改或覆盖用户全局 lazygit 配置。直接运行裸 `lazygit` 仍保持原样。

### AI commit

CLI 命令：

```bash
aiw commit --agent codex
aiw commit --agent codex --prompt "Use scope aiw"
aiw commit --agent codex --prompt-file ~/commit-style.md
aiw commit --print-prompt
aiw commit --dry-run
```

工作流：

1. 用户先用 lazygit 或 Git CLI stage 变更。
2. `aiw commit` 读取 staged diff、staged stat 和 git status。
3. 从 `config/commit-prompt.md` 构造基础 prompt。
4. 注入 `--prompt` 和 `--prompt-file` 的用户自定义要求。
5. 按 `config/agents.toml` 的 `commit_args` 调用目标 agent。
6. agent stdout 必须只返回 commit message。
7. 使用 `git commit -F -` 提交。
8. 如果 commit hook 失败，把失败 message 和 hook 输出注入下一轮 prompt，最多重试 `config/aiw.toml` 里的 `commit.retries` 次，当前默认 3。

### lazygit 内触发 AI commit

在 `aiw git` 打开的 lazygit 里：

```text
Ctrl-A
```

会触发全局 custom command：

```yaml
customCommands:
  - key: '<c-a>'
    context: 'global'
    description: 'AI commit staged changes'
    prompts:
      - type: 'input'
        title: 'Additional AI commit prompt (optional)'
        key: 'AIWCommitPrompt'
        initialValue: ''
    command: 'aiw commit --prompt {{.Form.AIWCommitPrompt | quote}}'
    output: terminal
```

建议使用方式：

1. `aiw git`
2. 在 lazygit 里 stage 文件或 hunk。
3. 按 `Ctrl-A`。
4. 可选输入额外 prompt；不需要就直接回车。
5. AIW 生成 commit message 并提交。

保留 lazygit 原生提交能力，不替换默认提交键。

## 验证记录

已验证：

```bash
npm run check
aiw doctor --gate git
aiw commit --prompt '' --print-prompt
```

验证结论：

- Node 语法检查通过。
- `aiw git` 的依赖门禁通过。
- 空自定义 prompt 不会破坏 commit prompt 构造。
- lazygit overlay YAML 可正常解析。
- 用户已手动验证 `aiw git` 的 delta diff 展示通过。

未自动验证：

- lazygit TUI 内 `Ctrl-A` 的完整交互流程。该流程需要真实 TTY 和用户操作；当前已从配置格式、命令和 CLI 侧验证，仍建议用户在真实 lazygit 里按一次确认。

## 已知风险和注意事项

- 某些 AIW checkout 可能没有 Git 远端；做 push 或 release 前需要先检查 `git status` 和 `git remote -v`。
- `fd` 缺失会影响 `aiw pick`。
- `cmux-git-diff` 缺失时右下 diff 会 fallback 到 delta。
- `aider` 缺失只影响选择 aider agent。
- cmux 配置不要贸然改。之前把 AIW 接入 `<code-root>/.cmux/cmux.json` 后，用户 reload cmux 发现原有入口异常，已回滚到原配置。
- lazygit `--config` 输出主要显示默认配置，不适合用来确认 `--use-config-file` 是否 merge 成功；更可靠的是实际从 `aiw git` 进入 TUI 验证。

## 后续待办

- 补 `fd`，让 `aiw pick` 完整可用。
- 再次评估 cmux 接入方式，优先做旁路入口或只读生成配置预览，不直接覆盖现有 cmux 配置。
- 给 `aiw commit` 增加更明确的成功/失败摘要，例如最终 commit hash。
- 给 `aiw commit` 增加非交互式测试脚本，覆盖 hook 失败重试、agent 失败、空 staged diff。
- 考虑增加 `aiw git --plain`，允许不加载 lazygit overlay。
- 考虑增加 `aiw handoff` 或 `aiw doctor --handoff`，输出当前配置、工具状态和常用命令。
- 后续如果交互需求变多，可把 Node CLI 继续扩展成 TUI，但当前不需要重构。
