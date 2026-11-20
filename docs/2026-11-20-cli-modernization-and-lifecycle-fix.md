# AIW CLI 现代化与工作区生命周期修复

日期：2026-11-20

## 现状与根因

- `lifecycleConfig()` 执行 `git show refs/heads/<base>:aiw.yaml`，`repoLifecycle()` 再用本地基线检查工作区仓库的声明和 remote/base。工作区未提交的脚本、新增仓库、已提交到 Change 分支的定义都可能被忽略或拒绝。只存在 `origin/main`，或远端跟踪分支领先本地 `main` 时，本地配置可能缺失或过时，影响 materialize、setup、cleanup、status 和 close。
- 手写路由、局部 `flag.NewFlagSet` 与 bash/zsh 静态补全各自维护命令面，已漏掉 version，也无法补全仓库及 Change。帮助和参数校验有时发生在项目发现之后。固定字符宽度的 `fmt.Printf` 会遇到长值及宽字符错位，直接读取 `os.Stdin` 的关闭确认无法可靠区分终端和管道。
- `syscall.Flock` 阻塞等待时缺少提示与取消。Git 子进程无调用上下文，非交互执行可能等待凭据输入。上次全量测试耗时 235.477 秒，中止时栈停在临时仓提交的 `exec.Cmd.awaitGoroutines`。宿主 Git 的签名和 fsmonitor 可启动继承管道的进程，测试须隔离这些配置并重新实测。
- YAML 反序列化默默接受拼错字段。原子覆盖把权限重置为临时文件的 0600。remote 归一缺少默认端口及多余尾斜杠处理，HTTP(S) 的 token 用户名未被拒绝。

## 目标与实施顺序

1. 直接使用当前工作区 `aiw.yaml` 校验选仓、解析业务仓基线、执行 setup/cleanup 并计算状态。新 worktree 从本机业务 store 的远端跟踪基线创建，已有 worktree 和业务分支保留原始创建 SHA。缩减范围先更新 `.aiw/change.yaml`，再物化。关闭基于已登记的 worktree，即使清单删除了原仓也能释放。
2. 持久化最近一次物化协调读取的 cleanup 命令。在仍声明该仓时优先使用当前配置，即使当前未定义 cleanup。配置删仓时使用已保存定义，旧文件缺少快照时依次搜索 `HEAD`、`refs/remotes/origin/<Base>`、`refs/heads/<Base>` 中的 `aiw.yaml`。无定义时正常释放。保留既有状态 `cleanup` 字符串和 JSON 输出结构，以独立可选键存放命令与快照存在信息，支持历史状态文件。释放优先使用登记的 Store，保留路径、Git common dir、分支、链接、未推送和待丢弃数据校验。
3. Git 由单次调用的上下文和输出控制，禁用 Git 终端口令提示，详细日志仅写 stderr，并对 URL 凭据和参数脱敏。测试用独立 Git 配置和明确的测试身份。项目/选择 YAML 严格检查字段，保留 `metadata` 扩展。原子写入保留普通文件权限，新文件为 0644，拒绝符号链接及非普通目标。registry 读取兼容旧 remote 键。
4. Cobra 命令树统一帮助、参数校验、全局选项、生成补全和动态候选。保留 `Run([]string) error` 与现有命令、错误退出和已有 JSON 结构。运行时目录通过调用对象传递，避免修改进程 cwd。帮助和参数错误无需访问 Git 或 AIW 数据。锁等待由信号上下文取消。
5. 终端按显示宽度排版，机器输出保持纯 JSON 或纯文本。TTY 下允许选择仓库、分支名确认和简短的仓登记进度提示。脚本使用直接 I/O。Doctor 区分失败、告警和信息，列出项目与本机工具检查。Change list 稳定排序并标注根目录失效。

## 依赖取舍

采用 `spf13/cobra` 复用命令树、帮助及四种 shell 的补全；`charmbracelet/lipgloss` 及其 table 统一处理终端列宽与语义样式；`charmbracelet/huh` 提供选择、确认及 spinner；`gofrs/flock` 保留现有锁文件和跨进程排他语义，并提供可取消等待；`golang.org/x/term` 检测真实终端。固定与仓库 Go 1.22 基线兼容的版本。

项目文件继续用 yaml.v3，配置入口和覆盖规则足够明确，无需 viper。Git 仍交由系统 Git，避免 go-git 重新实现 refspec、worktree、认证及配置语义。详细命令日志用标准库，无需 zap 或 zerolog。保留现有数据目录规则，无需 adrg/xdg。

## 兼容与数据安全

当前工作区的脚本定义属于可执行配置，执行前应信任工作区来源。历史快照只在仓库从工作区配置移除时用于释放。保留自动重试及显式确认的既有边界，repair 不重放脚本。物化不自动 fetch、clone、提交外部写入的选择清单或推送。CLI 不修改原始业务 checkout，也不拥有根 worktree 的删除权。保留旧状态字段和 Git 数据来源，现有业务分支不会因配置变更而移动。

TTY 交互要求输入、输出与错误流均为终端，`--no-input`、`--json` 或设置 CI 时禁用。颜色另外服从 `--no-color`、NO_COLOR、TERM=dumb 与输出端是否为 TTY。

## 验收

- 临时 Git 集成用例覆盖未提交/已提交的工作区定义、业务基线修改、仅远端跟踪的根基线、删除已物化仓库及旧状态回退。回归核对脏文件、远端确认、脚本中断及 owned worktree 校验。
- 用例覆盖帮助与参数校验、相对目录及全局选项位置、动态补全、JSON 列表及体检、严格 YAML、权限和锁等待取消。
- 执行 `go test -race -timeout 5m ./...`（记录实际用时）、`go vet ./...`、`scripts/build-release.sh`、生成补全的 zsh/bash 语法检查和 `git diff --check`。使用当前源码构建物验收，保留宿主安装版本。
