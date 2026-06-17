# aiw

`aiw` 是一个个人 AI 编程工作流 CLI。它把日常路径固定下来：开始隔离任务、打开适合 AI 协作的 workstation、审查改动、生成提交、清理 worktree。

默认文档语言是英文。英文版见 [README.md](./README.md)。

## aiw 是什么

AIW 是一个很薄的编排层。它负责决定工作流路径、检查依赖、读取配置、构造 prompt 和命令路由；可见界面和实际开发能力继续交给成熟工具。

- `aiw` 负责工作流决策、配置读取、依赖门禁、prompt、命令路由和 workstation layout plan。
- Worktrunk 负责 worktree 生命周期。
- tmux 默认承载 workstation panes，适用于普通终端、Ghostty、SSH 和无 GUI 开发机。
- cmux 是可选 runtime integration：只有 AIW 当前运行在 cmux 内，且 `cmux` CLI 可用时，AIW 才会打开 cmux workspace。
- lazygit 负责 Git TUI 操作。
- delta 负责 diff 渲染。
- agent CLI 负责模型交互。
- yazi、nvim、rg、fd、fzf、bat、eza 保持各自原生职责。

AIW 不应该变成终端模拟器、Git 客户端、编辑器、diff 查看器、守护进程、任务数据库或 agent 管理器。

## 快速上手

优先从已发布的 npm 包启动：

```bash
npx @chlrc/aiw init
aiw doctor
aiw new --agent codex
```

从本地 checkout 运行，主要用于开发 AIW 自身：

```bash
node bin/aiw --help
node bin/aiw doctor
node bin/aiw new --agent codex --dry-run
```

日常工作流通常是：

```bash
# 1. 检查本机工具链是否就绪。
aiw doctor

# 2. 创建或切换 Worktrunk worktree，并打开 runtime workstation。
aiw new --agent codex

# 3. 在标准 pane 里工作。
# Project: 顶部 Files + Agent，底部 Git。
# Scratch: Files + Agent。

# 4. 审查并 stage 改动。
aiw git

# 5. 基于 staged changes 生成 commit message 并提交。
aiw commit

# 6. feature worktree 完成后合并并清理。
aiw done dev --no-close-ui
```

兼容别名仍然可用：

```bash
aiw cmux-new --agent codex
aiw cmux scratch --agent codex
aiw done dev --no-close-cmux
```

新的文档、脚本和生成配置应优先使用 `aiw new`、`aiw scratch` 和 `--no-close-ui`。

## Workstation Runtime

AIW 支持两种 UI runtime：

- `tmux`：必需依赖。普通终端、Ghostty、SSH 和无 GUI 开发机里默认都使用 tmux。
- `cmux`：推荐但可选。只有当前进程运行在 cmux 内，且 `cmux` CLI 可用时，AIW 才使用 cmux。

用户不需要选择 backend。同一个命令会根据 runtime 自动决定打开 tmux session 还是 cmux workspace：

```bash
aiw layout --agent codex --dry-run
aiw scratch --agent codex --dry-run
```

依赖门禁里，`tmux` 是 blocking dependency，`cmux` 是 recommended dependency：

```bash
aiw doctor --gate new --agent codex
aiw doctor --gate layout --agent codex
aiw doctor --gate scratch --agent codex
```

如果缺少 `cmux`，`doctor` 会报告 recommended missing。真实运行 `aiw init` 时，需要 `--yes` 或交互确认才能跳过 cmux。

## 配置

AIW 按以下顺序读取配置：

1. 设置了 `$AIW_CONFIG_DIR` 时使用它。
2. 存在 `~/.config/aiw` 时使用它。
3. 否则使用仓库内默认 [config/](./config) 目录。

旧版本可能写入过 `[workstation]`。AIW 现在不再读取这个 section；runtime 会自动选择 tmux 或 cmux。

## 初始化

`aiw init` 会写个人配置，并可选注册 cmux actions：

```bash
npx @chlrc/aiw init --dry-run --yes
npx @chlrc/aiw init --cmux-scope home --yes
npx @chlrc/aiw init --cmux-scope none --yes
```

`tmux` 是必需依赖。`cmux` 是推荐依赖：当它不可用时，真实 init 需要 `--yes` 或交互确认后才会继续 tmux-only setup。

已有 AIW 配置默认保留。只有明确要替换时才使用 `--force`，替换前会创建备份。

## 常用命令

Workstation 和生命周期命令：

```bash
aiw doctor
aiw doctor --gate new --agent codex
aiw new --repo ~/Code/my-repo --branch feat/foo --agent codex --dry-run
aiw layout --agent codex --dry-run
aiw scratch --agent codex --root /private/tmp/aiw-sessions --id smoke --dry-run
aiw scratch list
aiw scratch close smoke --dry-run
aiw scratch gc --tmux --dry-run
aiw workspace list
aiw workspace open feat/foo --agent codex
aiw workspace gc --dry-run
aiw workspace done dev --agent codex --no-close-ui
```

Git 和本地工具界面：

```bash
aiw git
aiw diff
aiw diff --watch
aiw files
aiw edit src/file.ts:10
aiw grep keyword
aiw tree 3
```

`aiw git` 打开带 AIW overlay 的 lazygit。overlay 增加 `Ctrl-A`，用于对 staged changes 触发 AI commit，同时保留 lazygit 原生提交能力。

## 开发

代码改动后必须运行：

```bash
npm run check
```

Workstation / worktree 相关改动建议验证：

```bash
node bin/aiw doctor --gate new --agent codex
node bin/aiw doctor --gate layout --agent codex
node bin/aiw layout --agent codex --dry-run
node bin/aiw new --repo /private/tmp/repo --branch feat/foo --agent codex --dry-run
```

当前还没有完整自动化测试套件。风险较高的 CLI 行为建议在 `/private/tmp` 下创建临时 Git repo，并尽量用非交互命令验证。

## Agent Skills

AIW 按 npm `skills` CLI 可消费的 multi-skill 目录组织 skills：

- [skills/aiw-init](./skills/aiw-init/SKILL.md)：初始化和排查 AIW 环境。
- [skills/aiw-reference](./skills/aiw-reference/SKILL.md)：执行 AIW workstation、Git、commit、done、remove 和 GC 流程。

从仓库安装：

```bash
npx --yes skills add KiritoKing/aiw-cli --list -y
npx --yes skills add KiritoKing/aiw-cli --skill aiw-init -y
npx --yes skills add KiritoKing/aiw-cli --skill aiw-reference -y
```

`aiw init` 暂不安装这些 skills；它只初始化 AIW config 和可选 cmux 集成。
