# AIW 架构与开发门禁

## 代码分层

```text
cmd/aiw → internal/commands → internal/aiw → internal/gitcli
                                          → internal/localfs
                                          → skills
```

- `cmd/aiw`：只调用命令入口，统一报告错误与退出码。
- `commands`：Cobra 命令树、参数校验、帮助及补全。通过 `aiw.Service` 执行业务操作。
- `aiw`：项目配置、仓库登记、Change 物化/查询/释放和生命周期规则。Service 负责上下文发现、锁和递归门禁；内部状态函数保持私有。
- `gitcli`：系统 Git 调用、输出解析、错误脱敏，仅标准库。
- `localfs`：原子文件写入及文件锁，仅标准库。Change 的锁路径和操作顺序属于 aiw。
- `skills`：只提供 embed 资源，不依赖运行逻辑。

aiw 包中，project/selection/registry/state 分别对应项目清单、Change 选择、本机登记表和执行状态；paths 保存路径与 remote 规范化规则；service 是命令层入口。业务代码使用的 stdout/stdin 保留原有行为，此次分层没有引入全局 I/O 抽象。

## 导入规则

规则的机器来源是 `.harness/architecture/rules.json`。每个 Go 文件必须恰好属于一条规则，规则使用严格 allowlist：

| 范围 | 允许的非标准库导入 |
| --- | --- |
| cmd/aiw | commands |
| commands 生产代码 | aiw、Cobra |
| commands 测试 | aiw、Cobra、YAML（构造/读取临时仓 fixture） |
| aiw | gitcli、localfs、skills、YAML |
| gitcli / localfs / skills | 无 |
| harness 独立开发模块 | depguard 及用于运行分析器/匹配文件的直接依赖 |

没有旧 cli 文件例外，也没有全测试文件豁免。新建目录或更深的 package 需要先更新规则；未分类和被多条规则重复分类的文件都会失败。包导入使用精确名称或明确的子路径前缀，不用裸组织前缀放宽依赖。

Go 工具链负责禁止包循环依赖；harness 额外禁止未形成循环的跨层依赖，例如 commands 直接导入 gitcli、业务层引入 Cobra。

## 运行检查

```bash
scripts/check-architecture.sh
scripts/verify.sh
```

前者运行 harness 的自测并检查当前仓库，后者在架构检查之后继续运行测试、vet 和四平台构建。脚本可从任意工作目录启动；不安装全局工具，也不提供跳过架构检查的环境变量或选项。首次运行需要从正常 Go 模块代理下载开发工具依赖。

检查器在 `.harness/architecture` 独立 Go module 中固定 depguard v2.2.0，使用其规则匹配与诊断引擎。仓库适配负责严格读取配置、文件分类和向引擎传入 AST；不复写导入允许/拒绝算法。独立模块避免把检查器依赖加入 AIW 运行时模块，并保持 Go 1.22 基线。

采用 AST 文件扫描而非默认构建包加载，是为了同时检查测试、带自定义 build tag、非本机 GOOS/GOARCH 的源码。检查器不会执行或导入被检查的业务代码，也不尊重 `nolint` 注释。语法错误、空/错误配置、未知配置字段、覆盖缺口和扫描错误均失败退出；不会回退到工具默认配置。源文件使用相对仓库路径匹配，不依赖机器绝对路径。

标准库允许项展开时保持路径边界，避免把 `golang.org/...` 等名字误识别成 `go/...` 标准库。原始字符串/转义形式的导入先规范化，再交给 depguard 检查。源码符号链接不能成为绕过扫描的方式。

## CI 卡点与范围

CI verify job 在现有测试、vet、构建之前运行同一架构检查；任一步失败都会阻止 verify 成功。Release job 继续依赖 verify。此处落地的是仓库内的失败退出和 CI 接线；远端 required checks/分支保护属于管理员配置，不由源码文件自动保证。

检查器只约束静态导入关系。它不证明业务语义、安全性或目标系统运行行为；编译、业务测试、状态保护和交互验收仍分别提供证据。规则文件与 CI 本身的变更需要代码审核。
