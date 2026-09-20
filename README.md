# AIW

AIW 是一个 Go CLI，用于把多个业务仓库的 Git worktree 组装到 **Agent Repo 的现有 worktree** 中。Agent Repo 保存团队自行维护的跨仓上下文、规范和流程；Codex App、Paseo 或其他控制面负责创建 Agent Repo worktree 和运行 Agent。AIW 只负责业务仓登记、选择、物化、状态和释放。

## 工作模型

```text
Agent Repo（持久 Git 仓库，含 aiw.yaml）
  └─ Change worktree（由上层控制面创建）
       ├─ .aiw/change.yaml（Propose 阶段确定业务仓范围）
       └─ repos/
            ├─ web -> AIW 管理的 web worktree
            └─ api -> AIW 管理的 api worktree
```

AIW 不创建或删除 Agent Repo 的 Change worktree，不启动 Agent，不执行 OpenSpec 命令，也不管理终端面板、提交业务代码、合并分支或发布。业务仓 worktree 位于本机 AIW 数据目录；`repos/` 只有符号链接，不进入 Agent Repo 的 Git 历史。

## 安装与依赖

需要 Go 1.22+ 和 Git。开发仓库中运行：

```bash
go run ./cmd/aiw --help
go build -buildvcs=false -o ./build/aiw ./cmd/aiw
```

源码发布后可运行 `go install github.com/KiritoKing/aiw-cli/cmd/aiw@latest`。`scripts/build-release.sh` 在本地生成 macOS/Linux 的 arm64、amd64 二进制；脚本只构建，不上传。旧 npm 包的命令面不再由此仓库维护。

默认本机数据目录是 `~/.local/share/aiw`，可用 `AIW_HOME` 覆盖。Git 命令默认从 `PATH` 解析；需要指定 Git 可执行文件时设置 `AIW_GIT`。

### Shell Tab 补全

`aiw completion zsh` 和 `aiw completion bash` 输出对应的补全脚本，支持顶层及 `repo`、`change` 子命令和常用选项。在 zsh 中，将脚本放进 `fpath` 中已配置的补全目录，再打开新终端。例如使用 Oh My Zsh 默认目录时：

```bash
aiw completion zsh > ~/.oh-my-zsh/custom/completions/_aiw
```

如果 zsh 使用缓存式 `compinit`，在 `~/.zshrc` 的 `compinit` 或 Oh My Zsh 加载之后加上：

```zsh
autoload -Uz _aiw
compdef _aiw aiw
```

在 bash 中，可将 `source <(aiw completion bash)` 加到 `~/.bashrc`，或在当前会话中执行一次。

## 初始化固定 Agent Repo

```bash
aiw init ~/agent-repos/account-stack
```

`init` 只在显式指定的新空目录执行 `git init`，或接入已存在的 Git 仓库；它不会在非空的普通目录自动初始化 Git。命令生成 `aiw.yaml`、忽略 `/repos/` 的规则，并在 `AGENTS.md` 中写入 `<!-- aiw:start -->` / `<!-- aiw:end -->` 托管区块。区块说明多仓 Git 协作规则；用户自己的规则写在区块外。随后由你编辑并提交这些文件。
通过符号链接访问仓库也可以初始化，例如 macOS 的 `/tmp` 与 `/private/tmp`；AIW 会按真实路径核对 Git 根目录。

`init` 默认运行社区 `npx skills add`，从 [AIW 公开仓库](https://github.com/KiritoKing/aiw-cli)把 `aiw-init` 和 `aiw-reference` 安装到项目 `.agents/skills`。它使用项目级、非交互和复制安装；社区工具可能同时生成 `skills-lock.json`。缺少 Node 或 npx 时，Go 二进制使用构建时内置的两份 skill 兜底；已有不同内容的同名文件不会被覆盖。若 Node 可用但安装失败，`init` 会报告错误并保留已生成的 Agent Repo 文件，方便单独处理安装。安装其他工具或使用自定义分发方式时运行 `aiw init <path> --skip-skills`。已有 Agent Repo 可运行 `aiw agents sync`，只覆盖标记区块内的内容；标记损坏或 `AGENTS.md` 是符号链接时会拒绝写入。skill 的后续更新使用社区工具或自己的安装器，AIW 不维护更新器。

> 当前公开仓库的 skills 尚未发布本版 Go 工作流内容。AIW 会核对安装内容的工作流标记，发现旧版时让 `init` 报错；已生成的 Agent Repo 文件与社区工具写入的 skill 文件会保留，需检查后用自己的安装器替换。本地开发可用 `--skip-skills`，或在隔离项目中自行从当前源码安装。

```yaml
version: 1
name: account-stack
base: main
repos:
  web:
    remote: git@github.com:example/web.git
    base: main
    setup:
      command: ./scripts/aiw-setup.sh
      args: []
    cleanup:
      command: ./scripts/aiw-cleanup.sh
      args: []
  api:
    remote: git@github.com:example/api.git
    base: main
```

`aiw.yaml` 描述固定可用的仓库集合，不包含个人路径。`base` 是 Agent Repo 的基线分支；每个业务仓也有自己的基线分支。
每仓的 `setup`、`cleanup` 可选，命令及参数从 Agent Repo **基线分支已提交的** `aiw.yaml` 读取。命令在对应业务仓 worktree 内直接执行，不经过 shell；相对脚本路径相对该业务仓。执行时提供 `AIW_CHANGE_ROOT`、`AIW_CHANGE_BRANCH`、`AIW_REPO_NAME`、`AIW_REPO_PATH` 和 `AIW_LIFECYCLE_ACTION` 环境变量。脚本应支持重试；逐仓脚本不可递归调用 AIW Change 命令。

根仓的 SDD 或知识初始化由团队自己的提示词、Agent 和工具完成：它可以整理跨仓职责、候选仓库和需求上下文，再在本次 Change 的方案中确定要物化的仓库。`aiw.yaml` 仍只列可物化仓库；AIW 不要求 OpenSpec，也不替团队生成业务知识。

## 登记已有业务仓

```bash
cd ~/agent-repos/account-stack
aiw repo register web --source ~/Code/web
aiw repo scan ~/Code
aiw repo list
aiw repo list --json
aiw repo sync web
```

`register` 校验本地 checkout 的 `origin` 与清单 remote，再从它建立独立的 bare Git store。`scan` 递归查找已有仓库，遇到 `.git` 就停止扫描其子目录；同一 remote 匹配多个 checkout 时拒绝自动选择。原 checkout 不会被修改。`sync` 是显式网络 fetch；Change 物化不会隐式 fetch，也不会自动克隆缺失仓库。
`scan` 只匹配 `aiw.yaml` 已声明的仓库，不会修改该文件。如果 `repos: {}`，先填入各仓的 `remote` 和 `base`；空清单会返回明确错误，其他零匹配情况也会报告。
`repo list --json` 按仓库名返回 `name`、`remote`、`base`、`registered`；已登记仓库还包含本机 `source` 与 `store` 路径。remote URL 中的凭据会脱敏。命令只读取配置和本机登记表。

## 接入任意控制面

1. 在控制面中把 Agent Repo 作为 Project，并由控制面创建 Change worktree。该 worktree 刚创建时只有 Agent Repo 文件，没有业务仓。
2. 在 Propose 或等价方案阶段，由 OpenSpec、其他流程或人工在该 worktree 写入 `.aiw/change.yaml`：

   ```yaml
   version: 1
   repos:
     - web
     - api
   ```

3. 在任务分发前，由 Agent 从该 worktree 执行 `aiw change materialize`。AIW 为选中的仓库创建与 Agent Repo 当前分支**同名**的业务分支和 worktree，挂到 `repos/`，再逐仓执行 setup。全部成功后命令才返回就绪；AIW 不分发任务。
4. 执行中若发现仓库范围有误，修改 `.aiw/change.yaml` 并再次运行 `aiw change materialize`。每次物化都会重跑全部已选仓库的 setup。移出清单的仓库先执行 cleanup，再释放 worktree；有本地改动的仓库不会被静默删除。

`.aiw/change.yaml` 是控制面中立的输入文件。AIW 不解析 OpenSpec 的 proposal、design 或 tasks；这些流程自行决定何时生成、修改和提交选择清单。也可使用 `aiw change select --repo web --repo api`、`add-repo`、`remove-repo` 修改清单；AIW 只对自己修改的清单自动提交。已有同名业务分支若不属于当前登记的 Change，会报冲突。

常用只读命令：

```bash
aiw change status --json
aiw change list
aiw change path feat/example
aiw doctor
```

`change status` 汇总根仓与已挂入业务仓的 HEAD、分支、改动及 setup/cleanup 状态；JSON 中根仓的 `ready` 表示能否分发任务，业务仓的 `baseSha` 是创建 worktree 时保存的基线提交。setup 失败会保留 worktree 和链接，并使 Change 未就绪；修复环境后用 `aiw change setup` 续跑待执行或失败的仓库，或用 `aiw change setup <name>` 重跑指定仓库。仓库移出清单但仍有本地文件时，自动物化会先阻止释放；可检查文件后显式运行 `aiw change cleanup <name>`，再重试物化。`change open` 只返回控制面现有 worktree 路径。中断后 `aiw change repair` 恢复 worktree 和链接，不自动重放脚本；执行中的脚本结果未知，显式重试可能重复副作用。

根仓的 `git status` 和 `git diff` 不包含 `repos/` 链接内的业务改动。控制面可按需把具体业务仓路径作为本地工作区打开；也可直接运行 `git -C repos/<name> status` 和 `git -C repos/<name> diff`。各业务仓分别测试、提交、推送和创建 MR，根仓的 SDD 或交接记录可以汇总各仓提交 SHA、MR 链接及验证结果。AIW 只提供路径与状态，不执行代码交付。

## 释放业务 worktree

```bash
aiw change close
```

上层控制面的 cleanup script 调用 `aiw change close`，成功后才删除根 worktree。`close` 逐一读取 Agent Repo 和业务仓远端同名分支 SHA。远端不存在、不可达或与本地 HEAD 不一致时，会展示原因并要求再次确认。将被删除的业务 worktree 若含未提交、未跟踪或忽略文件，会单独列出并再次确认。无交互环境需要分别提供 `--allow-unpushed` 和 `--discard-changes`。AIW 逐仓执行 cleanup 后重新检查远端分支和待丢弃内容，才释放业务 worktree；cleanup 失败时保留 worktree，可运行 `aiw change cleanup <name>` 续跑。关闭保留所有分支及根 worktree；若关闭失败，控制面不得删除根 worktree。

## 开发验证

```bash
go test ./...
go vet ./...
scripts/build-release.sh
```

集成测试使用临时 Git 仓库，不访问真实业务仓。若系统 `git` 启动被 Xcode 许可状态阻断，可以给测试指定已安装的 Git，例如 `AIW_GIT=/Library/Developer/CommandLineTools/usr/bin/git go test ./...`。
