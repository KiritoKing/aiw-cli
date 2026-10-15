# AIW Cobra 重构报告

日期：2026-10-15。事前范围与兼容决策见 [迁移设计](2026-10-15-cobra-cli-design.md)。

## 结果

命令层已迁移到 Cobra v1.10.2：命令树同时驱动参数解析、帮助和 bash/zsh 补全。主要收益是减少重复维护、统一 CLI 契约，并把无效输入挡在业务上下文发现和加锁之前。Git 操作、配置格式和 worktree 生命周期继续使用现有实现。

## 有哪些实际优化

| 方面 | 迁移前 | 迁移后与收益 |
| --- | --- | --- |
| 命令维护 | 多层 switch、手写帮助、两份静态补全脚本 | 用 `cobra.Command` 声明命令；帮助和补全由同一命令树生成，删除 137 行静态脚本 |
| 帮助 | 顶层帮助与子命令处理不统一，可能先要求项目环境 | `aiw help repo register`、`aiw change close --help` 在非项目目录、无可用 Git、生命周期脚本环境中均可成功 |
| 参数校验 | 部分命令忽略剩余参数，某些错误先表现为缺少 aiw.yaml | 所有业务叶子命令声明位置参数约束，未知选项、缺少必需值和多余参数先返回参数错误 |
| 补全一致性 | 已实现的 version 未进入静态列表 | 实测补全协议能返回 version、close 风险选项；新增命令时无需再维护两份命令名清单 |
| 参数排列 | register 依赖 name 在选项之前，其他命令混用不同解析方式 | flag 可放在位置参数前后，统一支持长选项、布尔显式值和 `--` 分隔符 |
| 错误出口 | 各解析路径行为不统一 | Cobra 只返回错误，main 统一打印一次并退出 1；帮助成功退出 0 |
| 执行隔离 | 原手写解析没有命令对象复用问题 | 每次 Run 构建新命令树，防止迁移后引入 flag 状态泄漏；额外处理 Cobra 对 nil 参数的特殊约定 |
| 阅读结构 | 一个 cli.go 同时承载全部路由与多种操作 | 按 root/init/repo/change 拆分，命令声明和已解析参数的业务执行分开阅读 |

这里没有测量启动延迟或吞吐量，因此不宣称运行速度提升。收益集中在行为一致性、可维护性和可测试性。

## 已验证的行为对照

使用迁移前后分别构建的真实二进制，在临时非项目目录设置无效 `AIW_GIT`、不存在的 `AIW_HOME` 和 `AIW_LIFECYCLE_ACTIVE=1`：

| 调用 | 迁移前 | 迁移后 |
| --- | --- | --- |
| `aiw change close --help` | 返回 1，触发生命周期递归门禁 | 返回 0，显示 close 的帮助及选项 |
| `aiw help repo register` | 返回顶层总帮助 | 返回 register 专属帮助 |
| `aiw repo unknown` | 返回 aiw.yaml 不存在错误 | 返回未知 repo 子命令错误 |
| `aiw doctor extra` | 尝试执行 Git | 先拒绝多余参数 |
| `aiw change status` | 拒绝生命周期递归调用 | 仍拒绝递归调用 |

上述帮助与错误检查均没有创建 AIW_HOME。补全协议另外验证了 `aiw __complete ver` 和 `aiw __complete change close --dis`；这些调用返回候选和 Shell directive，不执行业务操作。

## 兼容性与代价

- 保留现有命令名、文档化选项、默认值、version 单行输出和 JSON 字段。业务分支、锁、dirty/unpushed 确认、setup/cleanup 与 repair 语义保持原有路径。
- `--repo` 使用 `StringArrayVar`，每次 flag 的值就是一个仓库名；`--repo web,api` 不会被自动按 CSV 拆开。临时仓测试确认无效逗号值不写选择清单，重复 `--repo` 正常工作。
- 有意改变帮助/错误文字、命令组裸调用行为及多余参数处理。旧脚本若依赖以前被忽略的参数或逐字匹配错误文案，需要调整。
- 直接依赖由 1 个增至 2 个，新增 Cobra；go.mod 另记录 pflag v1.0.9、mousetrap v1.1.0 两个间接依赖。未增加 Viper 或交互 UI 库。
- 同一 macOS arm64 主机、Go 1.26.5，以相同 `CGO_ENABLED=0 go build -buildvcs=false -trimpath` 参数构建：迁移前 **4,892,418 bytes**，迁移后 **5,428,306 bytes**，增加 **535,888 bytes（10.95%）**。这是本机未压缩二进制对照，不能外推到其他平台或归档大小。
- **Bash 补全新增 bash-completion 前提。** 在裸 Bash 中真实调用生成函数，得到 `_get_comp_words_by_ref: command not found`；这是 Cobra 生成脚本的依赖，已写入 README 和 `completion bash --help`。须安装并加载与 Bash 版本兼容的 bash-completion；本次未改动本机 Shell 环境。Zsh 沿用 compinit 流程。两种 Shell 都需让 aiw 在 PATH 中可执行，并重新生成旧补全。

## 学习 Go CLI 的建议阅读顺序

1. [main.go](../cmd/aiw/main.go)：只在最外层处理 stderr 和退出码。业务函数返回 error，便于直接测试而不终止测试进程。
2. [cli.go](../internal/aiw/cli.go)：`Run → newRootCommand → SetArgs → Execute`。每次构造新树，flag 状态由本次调用拥有。`Run(nil)` 被规范化为空 slice，避免 Cobra 改读进程参数。
3. [cli_repo.go](../internal/aiw/cli_repo.go)：看 `Use`、`Short`、`Args`、`Flags` 和 `RunE` 如何对应命令用法、帮助、校验和执行。对照 Node.js Commander 的 command/argument/option/action 很直接。
4. [cli_change.go](../internal/aiw/cli_change.go)：`Args` 负责纯输入约束；`PersistentPreRunE` 负责生命周期递归门禁；`changeAction` 统一发现 Change 并加锁；`RunE` 最终调用现有业务方法。帮助绕过这些业务执行步骤。
5. [completion.go](../internal/aiw/completion.go)：生成脚本只需要根命令对象，不再嵌入和同步静态命令名清单。
6. [cli_contract_test.go](../internal/aiw/cli_contract_test.go)：先看无项目环境下的契约测试，再看经公开 Run 入口执行的临时 Git 仓完整流程。

以 status 为例，核心声明是：

```go
var jsonOutput bool
status := &cobra.Command{
    Use: "status",
    Short: "Inspect the current Change and repository readiness",
    Args: cobra.NoArgs,
    RunE: changeAction(func(c ChangeContext, args []string) error {
        return c.status(jsonOutput)
    }),
}
status.Flags().BoolVar(&jsonOutput, "json", false, "JSON output")
```

`BoolVar` 把解析结果写入本次构造的局部变量；闭包读取这个变量调用业务方法。业务方法接收 bool，而不是再次解析命令行字符串。增加其他参数时，也沿用“先解析校验，再调用业务”的边界。

## 验收结果

| 检查 | 实际结果 |
| --- | --- |
| `go test -json ./...` | 通过；internal/aiw 32 个顶层测试，含子测试共 80 个 pass 事件，0 失败、0 测试跳过；包耗时 216.123 秒。cmd/aiw 与 skills 无测试文件 |
| `go vet ./...` | 通过 |
| `go mod verify` | all modules verified |
| `scripts/build-release.sh` | darwin/arm64、darwin/amd64、linux/arm64、linux/amd64 全部构建成功；输出定向至任务临时目录 |
| Shell 脚本与补全候选 | bash -n、zsh -n 通过；真实二进制 __complete 返回 version 和 close flags |
| `git diff --check` | 通过 |

复现时使用仓库现有验证命令即可：`go test ./...`、`go vet ./...`、`scripts/build-release.sh`。本次为避免写宿主缓存，将 GOCACHE、GOMODCACHE 和 AIW_DIST_DIR 指向了任务临时目录，结束时清理。

## 验收范围与后续边界

验收使用临时 Git 仓，不访问真实业务仓。保留既有 worktree、脏数据、未推送检查、范围调整和中断恢复测试；新增测试通过公开 Run 覆盖 register/list、重复 --repo、materialize/status/close、帮助/错误输出和 flag 隔离。

未在 Go 1.22 工具链上实际运行测试；go.mod 仍保留 go 1.22，所选 Cobra/pflag 的声明最低版本分别是 1.15/1.12。Linux 及 macOS amd64 只验证交叉编译，未在目标系统实际运行。补全完成了脚本语法与 Cobra 候选协议验证，未完成配置了 bash-completion 的真实 Bash Tab 或交互式 Zsh Tab 验收。

后续如需进一步降低测试中的全局状态，可单独把业务 stdout/stdin 和工作目录依赖显式注入；当前业务操作仍使用原有 os.Stdout、os.Stdin 和进程工作目录。本次未扩展到该重构，也未进行提交、推送、发布或本机全局安装。
