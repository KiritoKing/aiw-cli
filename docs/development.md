# AIW 当前开发上下文

本文记录当前 Go 实现的开发边界与交接约定。`docs/` 中已有日期的旧设计和工作日志是历史记录；判断现行命令行为时，以源码、测试和 `README.md` 为准。

## 当前架构

- 上层控制面创建 Agent Repo 的 Change worktree、运行 SDD 流程并分发任务。AIW 读取该 worktree 的 `.aiw/change.yaml`，在本机托管的业务仓中创建同名分支与 worktree，并在 `repos/` 下建立符号链接。
- Agent Repo 的初始 SDD 与跨仓知识由团队自己的提示词和工具维护；它们可以记录候选仓库，但只有 `aiw.yaml` 声明的仓库可被 AIW 物化。
- Agent Repo 的 `aiw.yaml` 声明可用仓库；业务仓本机注册信息及 Change 执行状态保存在 `AIW_HOME`。物化不隐式 fetch 或 clone，不修改作为注册来源的原 checkout。
- 每次物化在结构调整完成后执行选中仓库的 setup；全部成功才就绪。仓库移出范围或 `change close` 释放业务 worktree 前执行 cleanup。脚本定义取自 Agent Repo 基线分支已提交的 `aiw.yaml`。
- `change close` 由控制面的 cleanup script 调用。它负责业务 worktree 的检查和释放；只有成功后，控制面才能移除根 worktree。AIW 不接管根 worktree、SDD、Agent 或代码交付流程。
- `init` 维护根仓 `AGENTS.md` 中由 AIW 标记的协作规则，默认通过社区 `npx skills add` 安装 AIW skills；无 Node/npx 时仅用内置 Go 副本补齐缺失文件。已有仓库使用 `agents sync` 更新标记区块；skill 后续升级由社区工具或用户工具负责。
- `repo list --json` 和 `change status --json` 向控制面提供本机登记、业务仓分支与创建基线 SHA。根仓 Git 不追踪 `repos/` 链接内的业务改动；业务仓的 diff、提交、push、MR 均由各仓自身工具处理。

## 重大变更的事前文档

改变产品边界、CLI 契约、持久化格式、仓库/worktree 生命周期、数据清理或失败恢复语义，属于重大变更。开始实现前，先在 `docs/` 新建带日期的设计记录，写明当前行为、目标行为、受影响的命令与文件、兼容或迁移方式、数据安全边界以及验收方法。需要反映新的现行架构时，同时更新本文。设计记录须先于对应代码改动落地；实施中若决策变化，在文档中记录最终决策。已有日期的历史记录不回写，另建新记录承接变化。

常规修复或小幅文案调整可直接修改代码或文档，不需要额外设计记录。面向用户的行为变化在完成时同步更新 `README.md`。

## 每次变更的交接

每项变更实现并验证后、交付前更新仓库根目录的 `HANDOFF.md`，只保留当前活动的有效上下文：做了什么、为何这样做、验证结果、仍未完成或有风险的事项，以及下一位 Agent 需要继续的动作。不要把旧任务日志累计在 handoff 中；需要长期保留的过程记录写入新的带日期 `docs/` 文档。交接中区分已实现、已验证和未验证，且不得把本机安装或测试结果写成远端发布结果。
