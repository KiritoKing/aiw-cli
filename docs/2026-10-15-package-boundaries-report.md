# 目录迁移与架构 Harness 落地报告

日期：2026-10-15。按约定先完成迁移及业务回归，再接入架构检查。此前 Cobra 重构保留；本轮没有新增 CLI 运行时依赖。

## 目录与职责

```text
cmd/aiw/main.go                程序入口、错误输出与退出码
internal/
  commands/                   Cobra 命令树、参数、帮助与补全
  aiw/                        业务规则和 Service 操作入口
    service.go                上下文、锁与生命周期递归保护
    project.go                项目清单
    selection.go              Change 选择清单
    registry.go               本机仓库登记
    state.go                  Change 执行状态
    paths.go                  路径与 remote 规范化
    repo.go                   仓库登记、扫描、同步
    change.go / close.go      物化、修复、状态与释放
  gitcli/                     系统 Git 命令及结果解析
  localfs/                    原子写入与文件锁
skills/                       内嵌资源
.harness/architecture/        独立开发工具模块、规则与自测
scripts/check-architecture.sh 架构卡点
scripts/verify.sh             完整本地验证入口
```

依赖方向是 `cmd/aiw → commands → aiw → gitcli/localfs/skills`。命令通过 `aiw.Service` 操作业务，不访问登记文件或 Git。基础设施只依赖标准库；业务配置的 YAML 依赖留在业务层。CLI fixture 测试单独允许 YAML，不扩大生产代码权限。

没有创建泛化的 utils/shared 包。Git 和文件操作因职责稳定、与业务无关而独立；项目配置、Change 状态及其安全规则仍留在 aiw，避免为了文件数量拆出互相依赖的业务包。

## 优化与兼容

- Cobra 不再进入业务包；调整命令树不会迫使底层依赖命令框架。
- Service 统一上下文发现、仓库登记锁和 Change 锁；原有生命周期脚本递归门禁保留。list/path/open 继续走原查询路径。
- Git 封装继续执行系统 Git，保留 `AIW_GIT`、参数、输出解析与 URL 凭据脱敏，不引入另一套 Git 实现。
- 原子写入和 flock 的行为保持；本轮没有改变配置、JSON 或本机状态格式，无需用户数据迁移。
- `internal/aiw.BuildVersion` 链接注入地址保持，既有构建脚本继续有效。
- 测试随 package 迁移；命令契约测试通过公开 Run 入口，业务测试保留在业务层，避免反向导入。

## Harness 如何形成卡点

使用固定版本 depguard v2.2.0 的分析器，开发依赖放在独立 module。仓库适配层负责严格读配置、文件分类和扫描所有 Go AST；导入准入判断交给 depguard。

规则见 `.harness/architecture/rules.json`，共 8 个范围。每个 Go 文件必须恰好匹配一个范围；新目录、重复归属、配置缺失/损坏和工具执行失败都会失败退出。生产代码、测试、自定义 build tag 及非本机平台文件均检查；`nolint` 不提供绕过入口。

`scripts/check-architecture.sh` 先运行检查器自测，再扫描仓库；`scripts/verify.sh` 在它通过后才运行业务测试、vet 和四平台构建。CI verify 已接入同一脚本，release 仍依赖 verify。远端 required checks 与分支保护没有修改，托管 CI 尚未运行。

## 验证证据

- 完整根模块测试通过：37 个顶层测试，含子测试共 85 个 pass 事件，0 失败、0 测试跳过；无测试文件的包单独报告为无测试。
- 根模块 `go vet ./...` 与四平台构建通过：macOS/Linux × arm64/amd64。macOS arm64 产物实际执行 version，得到注入值 `v0.0.0-architecture`。
- 真实 CLI 在非项目目录、不可用 Git、生命周期标记环境下验证帮助、参数拒绝、递归保护与补全协议，未创建 AIW_HOME。
- 实际导入图读回符合单向依赖；根 go.mod/go.sum 与本轮迁移前的 Cobra 基线哈希一致。
- 架构检查通过当前 40 个 Go 文件、8 个范围；独立模块自测（7 个顶层测试、30 个子测试，0 失败、0 跳过）与 vet 通过。
- 实际检查器负例退出码均为 1：业务层导入 Cobra、带自定义 tag 的 Linux 命令文件直连 Git、无导入的新目录，以及错误/空/缺失配置。负例只写入隔离临时目录。另通过真实 `scripts/verify.sh` 入口注入命令层直连 Git，验证脚本在架构阶段退出 1，未继续运行后续步骤。
- 主验证使用 Go 1.26.5 / macOS arm64。另使用 Go 1.22.12 验证 CLI 定向测试及完整 harness 自测；该旧工具链默认链接产物在当前 macOS 上因缺少 LC_UUID 被 dyld 拒绝，使用 `-ldflags=-linkmode=external` 后测试通过。未改变项目默认链接参数，也未将此结果等同于 Go 1.22 全量业务测试。

架构检查约束静态导入关系，不替代业务语义、状态保护或交互测试。业务输出仍沿用现有 stdout/stdin，本轮未引入全局 I/O 注入；后续若需要并行运行命令或精细测试输出，可独立设计该边界。
