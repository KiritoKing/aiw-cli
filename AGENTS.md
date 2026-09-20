# AIW Agent Guide

本文件适用于整个 `aiw` 仓库。仓库当前实现是 Go CLI。开始非平凡修改前，先读 `README.md`、`docs/development.md` 和 `HANDOFF.md`，再核对相关源码与测试。`docs/` 下已有日期的旧设计稿是只读历史记录，不代表当前命令面。

## 产品边界

- 控制面（Codex App、Paseo 等）创建和关闭 Agent Repo 的 Change worktree；AIW 不接管其生命周期或 Agent session。
- Agent Repo 的 `aiw.yaml` 声明固定仓库集合。Change worktree 中的 `.aiw/change.yaml` 声明本次需要的子集。AIW 在方案确定后按清单物化业务仓，并允许再次物化以修正范围。
- AIW 只管理自己创建的业务仓 worktree 和 `repos/` 符号链接。原业务 checkout 不得被修改；Agent Repo 不包含业务代码、submodule 或 subtree。
- 控制面与 Agent 推进 Propose、物化和任务分发；`materialize` 仅在全部已选业务仓 setup 成功后视为就绪。控制面的 cleanup script 调用 `change close`，其成功后才删除根 worktree。AIW 在释放业务仓前执行逐仓 cleanup。
- 不实现 OpenSpec、Agent 控制、终端面板、业务提交、合并、push、PR 或发布流程。
- 根仓 `AGENTS.md` 的 AIW 托管区块仅讲多仓 Git 规则；用户规则保留在区块外。默认 skill 安装交给社区 `npx skills`，缺少 Node/npx 时才使用有限的 Go 兜底；AIW 不维护 skill 更新器。

## 实现位置

- `cmd/aiw/main.go`：CLI 入口。
- `internal/aiw/cli.go`：命令路由。
- `internal/aiw/config.go`：项目清单、Change 选择清单和本机状态。
- `internal/aiw/store.go`：本地仓登记、扫描和显式同步。
- `internal/aiw/change.go`、`close.go`：业务 worktree 物化、状态、修复和释放。
- `internal/aiw/lifecycle.go`：基线脚本定义、逐仓 setup/cleanup 及执行状态。
- `internal/aiw/git.go`：通过系统 Git CLI 执行 Git 操作。
- `internal/aiw/agent_context.go`、`skill_install.go`：托管区块同步及默认/兜底 skill 安装。
- `internal/aiw/completions/`：zsh、bash 补全脚本。
- `docs/development.md`：现行开发上下文和变更文档约定；`HANDOFF.md`：当前活动交接。

## 开发规则

- 使用 Go 1.22+；首版目标平台为 macOS 和 Linux。保持依赖少，不用 Go 库重写 Git 语义。
- 创建业务 worktree 前核对清单、注册表、基线和分支冲突。删除前核对登记路径、Git common dir 和分支；有本地数据时按 CLI 风险确认处理。
- 不在物化过程中隐式 fetch、clone、push 或提交外部流程写入的 `.aiw/change.yaml`。AIW 自己修改选择清单时只提交该文件。
- 更新 Agent Repo 规则时只替换完整的 `<!-- aiw:start -->` / `<!-- aiw:end -->` 区块，保护区块外字节；兜底 skill 安装不覆盖已有不同内容的文件。
- 逐仓 setup/cleanup 命令取自基线分支已提交的 `aiw.yaml`，在业务仓 worktree 中执行。持久化脚本状态；脚本中不得递归调用 AIW Change 命令。`repair` 不自动重放结果未知的脚本。
- 重大变更开始实现前，先在 `docs/` 新建带日期的设计记录，说明现状、目标、影响范围、兼容或迁移方式、数据安全边界和验收方法；产品边界、CLI 契约、持久化格式、worktree 生命周期或失败恢复语义变化都适用。现行架构有变化时同步更新 `docs/development.md`。已有日期的历史记录不回写。
- 每次变更实现并验证后、交付前都更新根目录 `HANDOFF.md`，只保留当前活动的有效上下文、验证证据与剩余事项；长期工作日志另存为新的带日期 `docs/` 文档。面向用户的行为变化同时更新 `README.md`。
- 保留无关的本地改动。
- 不启动开发服务器。

## 验证

```bash
go test ./...
go vet ./...
scripts/build-release.sh
```

`internal/aiw/integration_test.go` 使用临时 Git 仓验证控制面先创建根 worktree、AIW 后物化和关闭的流程。必要时以 `AIW_GIT` 指定可运行的 Git 路径。测试需同时覆盖正常流程、仓库范围变更、重复扫描、未推送与脏 worktree，以及中断恢复。
