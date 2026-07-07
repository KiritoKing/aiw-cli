# AIW 命令参考 - 2026-06-18

本文档记录当前 checkout 的 AIW 命令面。`aiw migrate` 已删除；旧 `[workstation]` 配置不再读取。

## 主命令

| 命令 | 别名 | 作用 | 关键参数 / 行为 |
|---|---|---|---|
| `aiw help` | `-h`, `--help` | 打印总帮助 | 只读 |
| `aiw init` | 无 | 初始化个人 AIW 配置，可选注册 cmux actions | `--cmux-scope home\|code\|none`, `--config-dir`, `--code-root`, `--worktrees-root`, `--sessions-root`, `--launcher`, `--force`, `--yes`, `--dry-run`, `--no-reload`, `--json` |
| `aiw doctor` | 无 | 检查依赖和 agent 可用性 | `--json`, `--gate <p0\|init\|new\|layout\|scratch\|scratch-resume\|workspace\|worktrunk\|diff\|commit>`, `--agent <name>` |
| `aiw new` | `aiw cmux-new`, `aiw cmux new` | 选择或创建 Worktrunk worktree，并打开 AIW workstation | `--repo`, `--branch`, `--base` / `--from`, `--agent`, `--pick-repo`, `--create`, `--local`, `--dry-run` |
| `aiw layout` | 无 | 在当前目录打开 project workstation，不创建 worktree | `--agent`, `--print-json`, `--dry-run` |
| `aiw diff` | 无 | 查看 diff | 默认优先 `cmux-git-diff`，否则 `git diff \| delta`；支持 `--watch`, `--staged`, `--all` |
| `aiw git` | 无 | 打开 lazygit，并注入 AIW lazygit overlay | 透传 lazygit 参数 |
| `aiw commit` | 无 | 基于 staged diff 生成 commit message 并提交 | `--agent`, `--prompt`, `--prompt-file`, `--retries`, `--dry-run`, `--print-prompt` |
| `aiw commit-message` | 无 | 只生成 commit message，不提交 | `--agent`, `--prompt` |
| `aiw files [path]` | 无 | 打开文件管理器 | 默认 `yazi` |
| `aiw edit <file[:line]>` | 无 | 打开编辑器 | 默认 `nvim`；支持 `file:line` |
| `aiw grep <query>` | 无 | `rg + fzf + bat` 搜索并跳编辑器 | 回车打开命中位置 |
| `aiw pick` | 无 | `fd + fzf + bat` 选文件并打开编辑器 | 依赖 `fd`, `fzf`, `bat`, editor |
| `aiw tree [depth]` | 无 | 打印目录树 | 优先 `eza --tree`，否则 `find` |

## Scratch 命令

| 命令 | 别名 | 作用 | 关键参数 / 行为 |
|---|---|---|---|
| `aiw scratch [id]` | `aiw session`, `aiw cmux scratch` | 创建非项目 scratch session，并打开 Files + Agent panes | `--agent`, `--root`, `--id`, `--message`, `--dry-run` |
| `aiw scratch resume` | `aiw scratch open` | 选择并重新打开已有 scratch session | `--agent`, `--root`, `--id`, `--query`, `--dry-run` |
| `aiw scratch list` | `aiw scratch ls` | 列出 scratch sessions，并显示 AIW-managed UI 状态 | `--root`, `--json`；JSON 包含 `open`, `uiImplementation`, `uiRef` |
| `aiw scratch close` | 无 | 关闭某个 scratch 的 tmux/cmux UI | `id\|path`, `--root`, `--dry-run`, `--json`；不删除 scratch 目录 |
| `aiw scratch gc --tmux` | `aiw scratch clean --tmux` | 清理长期未使用的 AIW-managed scratch tmux session | `--root`, `--stale-seconds`, `--dry-run`, `--apply` / `--yes`, `--json`；只关闭 stale、unattached、`kind=scratch` 的 tmux session，不删除 scratch 目录 |

## Workspace 命令

| 命令 | 别名 | 作用 | 关键参数 / 行为 |
|---|---|---|---|
| `aiw workspace list` | `aiw ws list`, `aiw ws ls`, `aiw ws als`, `aiw workspace status`, `aiw list`, `aiw ls`, `aiw als` | 列出 worktrees/workspaces，包含 dirty、age、GC、UI 状态 | `--json`, `--color`, `--stale-seconds` |
| `aiw workspace open [target]` | `aiw ws open`, `aiw workspace switch`, `aiw open`, `aiw switch` | 打开指定 worktree/branch，或进入 picker | `--agent`, `--remotes`, `--branches`, `--worktrees-only` |
| `aiw workspace done [target]` | `aiw ws done`, `aiw done` | 合并当前 feature worktree、清理并默认关闭 UI | `--agent`, `--retries`, `--no-close-ui`, `--close-ui`；`--no-close-cmux` 为兼容旧名 |
| `aiw workspace remove` | `aiw ws remove`, `aiw workspace rm`, `aiw remove` | 删除 worktree，先做 dirty 检查，成功后关闭对应 AIW UI | 透传 `wt remove` 参数；支持 `--force`, `--dry-run` 等传入 |
| `aiw workspace gc` | `aiw ws gc`, `aiw clean`, `aiw gc`, `aiw workspace clean` | 预览或清理 safe worktrees；stale warning 不自动删 | `--dry-run`, `--apply` / `--yes`, `--json`, `--stale-seconds` |
| `aiw workspace states` | `aiw workspace state`, `aiw ws states` | 解释 workspace state 含义 | 只读说明 |
| `aiw workspace help` | `aiw ws help` | 打印 workspace 子命令帮助 | 只读 |

## Runtime 规则

| 场景 | 行为 |
|---|---|
| 普通终端 / Ghostty / SSH / 无 GUI | 默认走 `tmux` |
| 当前进程在 cmux runtime 内且 `cmux` 可用 | 走 `cmux` |
| AIW-created tmux session | 写入 `@aiw_managed`, `@aiw_cwd`, `@aiw_kind`, `@aiw_name`，并 per-session `mouse on` |
| tmux 生命周期 | `done`, `remove`, `gc`, `scratch close`, `scratch gc --tmux` 负责显式关闭；普通 detach 不自动杀 session |

