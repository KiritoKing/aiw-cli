# Package 分层迁移与架构门禁设计

日期：2026-10-15。按用户指定顺序，先迁移，再接入 harness。

## 当前行为与目标

Cobra 命令定义与业务代码当前共处 internal/aiw。将命令集中到 internal/commands，业务入口与规则保留 internal/aiw，系统 Git 封装提取至 internal/gitcli，原子文件写入与文件锁提取至 internal/localfs。按职责拆分 config.go。现有 Cobra 重构及用户改动均保留。

依赖方向为 cmd/aiw → commands → aiw → gitcli/localfs；aiw 可读取 skills 内嵌资源与 YAML 库，commands 使用 Cobra，基础设施层只使用标准库。

## 迁移范围

- main 改调 commands.Run，保留 stderr 和退出码语义；版本注入仍使用 internal/aiw.BuildVersion 的现有链接地址。
- commands 只处理命令声明、解析、参数校验与展示适配；aiw.Service 提供操作入口，封装上下文发现、锁及生命周期递归门禁，避免命令层接触持久化细节。
- 原初始化、仓库列表和 Change 查询业务迁回 aiw；命令构造函数无需访问未导出内部实现。
- 业务函数与 Git 调用保持原有操作参数、输出和错误脱敏；文件锁顺序、cleanup、dirty/unpushed 确认和 repair 语义不变。
- 单元测试跟随所属 package；CLI 黑盒测试通过 commands.Run，业务测试通过业务入口/内部函数初始化，避免测试反向 import 形成循环。

## Harness

迁移验收后接入基于 depguard 的开发检查，固定工具版本，独立于运行时模块。规则覆盖生产代码与测试；未知目录不得借未分类位置跨层导入。配置错误、空配置或执行失败必须失败退出。禁止通过全目录例外放行 Cobra。

检查器需覆盖不同平台及带 build tag 的 Go 源文件，并以合法对照与真实违规样例验证参数别名、空白导入、跨层导入、业务层依赖 Cobra 和基础设施反向依赖。将同一检查接入本地验证入口与 CI verify，保留现有测试、vet、构建及 release 依赖。

## 兼容与安全

保持现有 CLI、JSON、配置/状态格式及 Go 1.22 基线；没有用户数据迁移。此次仅调整本地源码与仓库 CI 定义，不修改远端分支保护、真实业务仓、用户 Shell、全局工具安装或发布状态。

## 验收

迁移后执行 Go 测试、vet、四平台构建、版本注入及 CLI smoke。随后执行 harness 合法/违规/配置故障测试与仓库全量扫描。以包依赖图读回确认真实单向依赖，更新开发上下文、README、AGENTS 和 HANDOFF。旧日期设计及 Cobra 报告保留为对应阶段历史证据。
