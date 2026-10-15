# AIW 当前开发上下文

本文记录当前 Go 实现的开发边界与交接约定。`docs/` 中已有日期的旧设计和工作日志是历史记录；判断现行命令行为时，以源码、测试和 `README.md` 为准。

## 当前架构

- 命令层位于 `internal/commands`，由 Cobra 构建，每次 `Run` 新建根命令；root/init/repo/change/agents/doctor/completion 按命令归组。参数校验先于业务执行，帮助不依赖项目或 Git；main 统一输出错误及设置失败退出码。
- `internal/aiw.Service` 是命令层调用的业务入口，负责项目/Change 上下文、仓库/Change 锁和生命周期递归门禁。项目、选择清单、登记表与执行状态按职责拆文件；业务规则仍在同一 aiw package，业务输出与交互保持原有 stdout/stdin 路径。
- 依赖方向是 `cmd/aiw → commands → aiw → gitcli/localfs`。`gitcli` 负责系统 Git 调用、输出解析和错误脱敏，`localfs` 负责原子写入与文件锁；两者只依赖标准库。Cobra 只属于命令层，YAML 与内嵌 skill 资源属于业务层。
- `.harness/architecture` 使用独立 Go module 固定 depguard 开发依赖，规则见其 `rules.json`。`scripts/check-architecture.sh` 先跑门禁自测再扫描全部 Go 源文件；配置、覆盖或依赖错误均阻断。`scripts/verify.sh` 串行执行架构、测试、vet、四平台构建，CI verify 在现有测试前调用同一架构入口。测试没有整体豁免，CLI fixture 的 YAML 依赖单独声明。详细约束见 `docs/architecture.md`。
- 上层控制面创建 Agent Repo 的 Change worktree、运行 SDD 流程并分发任务。AIW 读取该 worktree 的 `.aiw/change.yaml`，在本机托管的业务仓中创建同名分支与 worktree，并在 `repos/` 下建立符号链接。
- Agent Repo 的初始 SDD 与跨仓知识由团队自己的提示词和工具维护；它们可以记录候选仓库，但只有 `aiw.yaml` 声明的仓库可被 AIW 物化。
- Agent Repo 的 `aiw.yaml` 声明可用仓库；业务仓本机注册信息及 Change 执行状态保存在 `AIW_HOME`。物化不隐式 fetch 或 clone，不修改作为注册来源的原 checkout。
- 每仓可选的 `metadata` 是项目自定义的静态信息，AIW 解析并经 `repo list --json` 提供，但不据此改变 Git、worktree 或脚本行为。仓库 ID、remote 和 base 只在 `aiw.yaml` 维护。
- 每次物化在结构调整完成后执行选中仓库的 setup；全部成功才就绪。仓库移出范围或 `change close` 释放业务 worktree 前执行 cleanup。脚本定义取自 Agent Repo 基线分支已提交的 `aiw.yaml`。
- `change close` 由控制面的 cleanup script 调用。它负责业务 worktree 的检查和释放；只有成功后，控制面才能移除根 worktree。AIW 不接管根 worktree、SDD、Agent 或代码交付流程。
- `init` 维护根仓 `AGENTS.md` 中由 AIW 标记的协作规则，生成 `.aiw/aiw-toolchain.env` 和 `.aiw/bootstrap-aiw.sh`，默认通过社区 `npx skills add` 安装 AIW skills；无 Node/npx 时仅用内置 Go 副本补齐缺失文件。版本锁必须由团队填写制品地址和 Release 校验和后提交；bootstrap 在校验完成前不下载。已有仓库使用 `agents sync` 更新标记区块；skill 后续升级由社区工具或用户工具负责。
- `repo list --json` 和 `change status --json` 向控制面提供本机登记、业务仓分支与创建基线 SHA。根仓 Git 不追踪 `repos/` 链接内的业务改动；业务仓的 diff、提交、push、MR 均由各仓自身工具处理。
- `.github/workflows/ci.yml` 在 PR/master 上验证 Go 测试、vet 和四个平台的交叉编译；只有 `v*` tag 会创建 GitHub Release，并附带版本化归档和 SHA-256 清单。内部制品库应镜像这些不可变制品，不能由该公开工作流推断或管理。

## 重大变更的事前文档

改变产品边界、CLI 契约、持久化格式、仓库/worktree 生命周期、数据清理或失败恢复语义，属于重大变更。开始实现前，先在 `docs/` 新建带日期的设计记录，写明当前行为、目标行为、受影响的命令与文件、兼容或迁移方式、数据安全边界以及验收方法。需要反映新的现行架构时，同时更新本文。设计记录须先于对应代码改动落地；实施中若决策变化，在文档中记录最终决策。已有日期的历史记录不回写，另建新记录承接变化。

常规修复或小幅文案调整可直接修改代码或文档，不需要额外设计记录。面向用户的行为变化在完成时同步更新 `README.md`。

## 每次变更的交接

每项变更实现并验证后、交付前更新仓库根目录的 `HANDOFF.md`，只保留当前活动的有效上下文：做了什么、为何这样做、验证结果、仍未完成或有风险的事项，以及下一位 Agent 需要继续的动作。不要把旧任务日志累计在 handoff 中；需要长期保留的过程记录写入新的带日期 `docs/` 文档。交接中区分已实现、已验证和未验证，且不得把本机安装或测试结果写成远端发布结果。
