# Agent Repo 上下文与交付信息扩展

日期：2026-09-20

## 现状与目标

AIW 的 `init` 只生成 `aiw.yaml` 和忽略规则。根仓没有 AIW 托管的 Agent 规则；本机登记只支持文本列表，Change 状态 JSON 未暴露创建时的业务仓基线 SHA。现有 AIW skills 位于源码仓库，未接入新建 Agent Repo。目标是补齐这些辅助信息，同时保持 AIW 只组织多仓 Git worktree 的边界。

## 行为与接口

- `init <path> [--skip-skills]` 写入可提交的 `AGENTS.md` 托管区块，并默认从公开 AIW Git 仓库调用社区 `npx skills add`，以项目级、非交互、复制方式安装两个 AIW skills。`--skip-skills` 允许用户自行选择安装器。
- `agents sync` 在已有 Agent Repo 中创建、追加或替换 `<!-- aiw:start -->` 和 `<!-- aiw:end -->` 之间的规则。区块外字节不变。标记不完整、重复、乱序，或文件为符号链接时拒绝写入。
- Node 或 npx 不存在时，内置 Go 副本只安装不存在的 AIW skill 文件。同名目标有不同内容时报错且保留用户文件；不实现技能更新器。Node 可用但社区安装失败时报告失败，不切换安装器。
- `repo list --json` 返回固定清单中每仓的 remote、base、本机登记状态、source 与 store；`change status --json` 在每个业务仓行增加创建时保存的 `baseSha`，其余字段不变。

## 兼容、数据安全与发布

既有 `init` 在 `aiw.yaml` 已存在时仍拒绝覆盖；已有 Agent Repo 使用 `agents sync`。新 `AGENTS.md` 文件可由用户提交，既有个人规则仅在标记区块内被覆盖。技能安装前检查同名路径，避免接管已有用户文件；安装失败后保留已生成的根仓配置并报告后续手动操作。公开仓库目前仍是旧版 Node 技能说明，安装后核对 Go 工作流标记，拒绝把旧版当成成功；默认远端安装在发布新版技能并读回之前不能算通过验收。

## 验收

用临时 Git 仓验证 AGENTS 区块创建、更新、边界保留与拒绝条件；用受控 npx 程序验证调用参数，用隔离 PATH 验证 Go 兜底和冲突行为；核对 JSON 输出与历史字段。运行 `go test ./...`、`go vet ./...` 并构建 macOS/Linux 二进制。业务仓 diff、提交、push、MR 和 SDD 流程继续由业务工具与控制面负责。
