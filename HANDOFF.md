# AIW Handoff

更新时间：2026-10-15

## 当前活动

已按用户指定顺序完成 package 迁移，再落地架构 harness；保留此前 Cobra v1.10.2 重构。此次实现与本地验收已完成，按用户要求拆分提交并推送当前 lovely-buffalo 分支；实际提交与推送结果以 Git refs 为准。没有发布或替换全局 aiw。

当前架构与规则说明见 `docs/architecture.md`，本轮设计与报告见 `docs/2026-10-15-package-boundaries-design.md`、`docs/2026-10-15-package-boundaries-report.md`。此前带日期的 Cobra 文档保留为历史记录。

- 单向依赖：`cmd/aiw → internal/commands → internal/aiw → internal/gitcli/internal/localfs/skills`。
- commands 负责 Cobra 命令、参数、帮助和补全；通过 aiw.Service 调业务。Service 统一业务上下文、登记锁/Change 锁和生命周期递归保护；list/path/open 保留查询路径。
- Git 与文件操作移入基础设施包；系统 Git 语义、AIW_GIT、凭据脱敏、原子写入与文件锁行为保留。项目清单、选择清单、登记和执行状态分别拆为 project/selection/registry/state 文件。
- harness 位于 `.harness/architecture` 独立 Go module，固定 depguard v2.2.0。根运行时 go.mod/go.sum 与本轮开始时的 Cobra 基线一致。
- `scripts/check-architecture.sh` 自测后扫描全部 Go 文件；严格 allowlist、唯一归属、覆盖测试/build tag/异平台源码，配置与扫描错误失败退出。`scripts/verify.sh` 在架构检查后串行执行业务测试、vet 与构建。
- CI verify 在业务测试前运行相同架构脚本；release 继续依赖 verify。README、AGENTS 与开发上下文已同步。

## 已完成的验证

- 完整根模块 `go test -json ./...`：37 个顶层测试，含子测试共 85 个 pass 事件，0 失败、0 测试跳过；cmd/aiw 和 skills 无测试文件。
- 根模块与 harness 的 `go vet ./...`、四平台 `scripts/build-release.sh`、脚本语法和 `git diff --check` 通过。macOS/Linux × arm64/amd64 构建成功；本机产物 version 实际输出注入值 `v0.0.0-architecture`。
- CLI 在非项目目录、无可用 Git、生命周期环境下验证帮助、参数拒绝、递归保护与补全协议，未创建 AIW_HOME。
- 实际 package 导入图符合分层；架构扫描通过当前 40 个 Go 文件、8 个范围。独立模块自测 7 个顶层测试、30 个子测试全部通过、无跳过。独立检查器包含合法依赖、各层越界、测试越界、别名/空白/原始字符串/转义导入、未知目录、规则重叠、配置故障、build tag、平台文件、nolint 和符号链接负例。
- 隔离目录执行真实检查器：业务引入 Cobra、带 tag 的 Linux 命令直连 Git、未知目录、损坏/空/缺失配置均失败退出。负例未修改真实业务源码；隔离副本的真实 verify.sh 同样在架构阶段返回 1，没有继续执行后续检查。
- 主工具链为 Go 1.26.5 / macOS arm64；另在 Go 1.22.12 下用 external link mode 跑通 CLI 定向测试及完整 harness 自测。

## 兼容与未验证项

- CLI、JSON、配置和本机状态格式保持，没有用户数据迁移。业务层仍沿用现有 stdout/stdin；尚未引入统一 I/O 注入。
- Bash 生成补全要求预先加载 bash-completion；未修改用户 Shell，也未验收交互式 Bash/Zsh Tab。
- 当前 macOS 的 dyld 拒绝 Go 1.22 默认链接产物（缺 LC_UUID）；定向兼容性验证使用 `-ldflags=-linkmode=external`，未修改项目默认参数。未跑 Go 1.22 全量业务测试；其他平台只交叉编译。
- 托管 CI 尚未运行，远端 required checks/分支保护未修改。仓库门禁能失败退出，但规则与 CI 本身仍需要代码审核。
