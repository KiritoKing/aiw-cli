# 仓库元数据扩展

日期：2026-09-20

## 现状与目标

AIW 的 `aiw.yaml` 目前只承载 Git 组装字段。团队可能另建业务仓描述清单，导致仓库 ID、remote、base 双重维护。本次允许每仓在 `aiw.yaml` 内附带可选的 `metadata` 映射，使一个 Agent Repo 可以用单一仓库清单承载选仓线索。AIW 仍只根据 `remote`、`base` 和生命周期脚本执行 Git 操作。

## 契约与兼容

- `repos.<name>.metadata` 为可选 YAML 对象，内容由项目自行定义；旧 `aiw.yaml` 无需迁移，`version` 保持 1。
- `metadata` 随配置解析，并在 `repo list --json` 原有字段外按需返回；AIW 不解释其中的 PSM、技术栈、标签或运行时值，不以其决定物化范围或执行脚本。
- `metadata` 不得放密钥或本机路径。`repo list --json` 会原样显示它；项目应自行管理敏感信息。
- 生命周期脚本仍从 Agent Repo 已提交的基线配置读取；当前 Change 的业务元数据编辑不会隐式改变脚本。Git、worktree 和失败恢复语义不变。

## 验收

测试旧配置兼容、metadata 解析/序列化及 `repo list --json` 输出；运行 `go test ./...`、`go vet ./...` 和 macOS/Linux 构建。项目层面的旧清单删除和消费方迁移由该项目的独立设计记录承接。
