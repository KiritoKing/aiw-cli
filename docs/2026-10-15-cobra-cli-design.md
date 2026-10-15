# Cobra 命令层迁移设计

日期：2026-10-15。本文在实现前建立，最终验收另见同日期迁移报告。

## 现状与目标

当前 `Run` 以 switch 分发命令，各 handler 混用手工解析和标准库 flag；帮助文本与 bash/zsh 补全各自维护。部分命令未检查剩余位置参数，帮助请求可能先进入项目发现或 Change 上下文构建。已有 `version` 未进入静态补全列表。

迁移到 Cobra 的声明式命令树，以同一份命令、参数和 flag 定义生成帮助及补全。只引入 Cobra 和必要传递依赖，不引入配置框架或交互 UI。保留 Go 1.22 最低版本。

## 影响范围与实现

- `Run(args []string) error` 及 main 的错误出口保留，每次执行新建命令树，避免 flag 状态泄漏。
- 按 root/init/repo/change/completion 拆分命令层；将原业务执行代码改为接收已解析的参数，不保留第二套手写路由。
- 叶子命令声明 `Args` 校验；未知 flag、位置参数数量和必需 flag 在业务上下文发现、加锁、文件写入之前检查。
- 帮助、版本、补全不要求 Agent Repo、Git 或 AIW 数据目录；错误只由 main 打印一次，失败保持非零退出码。
- `completion bash|zsh` 保留，由 Cobra 从根命令生成；移除旧静态脚本，避免继续维护重复清单。不扩大支持的 Shell 范围。
- `change` 实际执行仍拒绝生命周期脚本递归调用；需要当前 Change 的命令继续统一加锁。查询 list/path/open 保持原有无 Change 锁路径。

## 兼容与有意变化

保持现有命令名、flag 名、默认值、JSON 结构、版本单行输出和正常业务语义。`--repo` 使用 StringArray，保持重复 flag 每次值为一个仓库名，不把逗号拆成多个值。

有意改变：生成式帮助文本；各级 `--help` 和 `help <command>` 返回成功；命令组裸调用显示帮助；所有叶子命令拒绝多余参数；flag 可放在位置参数前后并支持标准布尔值写法。错误文字采用 Cobra 约定，不承诺逐字兼容。未知子命令不得退回成功帮助。旧补全需重新生成；已文档化的加载路径保留。

实施验证补充：Cobra 的 Bash 生成脚本依赖已加载的 bash-completion（提供 `_get_comp_words_by_ref`）。裸 Bash 运行验证暴露了这个新增前提，README 与 `completion bash --help` 必须明确说明；不以自写通用补全兼容层掩盖依赖，也不修改用户的 Shell 环境。Zsh 使用其原有 compinit 加载流程。

## 数据安全边界

不改变 Git 命令实现、配置/状态格式、setup/cleanup 重放语义、dirty/unpushed 确认和 worktree 释放逻辑。不访问真实业务仓验收，不 fetch/push/发布，不替换本机安装。测试只在临时仓库运行。

## 验收方法

- 命令契约测试：各级帮助、未知命令/flag、缺失或多余参数、必需值、重复运行无 flag 泄漏、生命周期递归门禁。
- 生成补全：bash/zsh 成功生成并通过对应 shell 语法检查；从命令树完成 version 和 close flags，补全路径无业务副作用。
- 经公开 Run 入口验证临时仓 repo register/list 与 Change select/materialize/status/close，检查 JSON 和现有生命周期保护。
- 执行 `go test ./...`、`go vet ./...`、`scripts/build-release.sh` 及 `git diff --check`；记录实际工具链，区分交叉编译和目标系统运行。
- 报告列出架构和行为改善、依赖及构建体积取舍、源码阅读路径、验证证据及未验证项。同步 README、开发上下文和 HANDOFF。
