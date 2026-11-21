# AIW Handoff

2026-11-20 · `silly-stingray`

## 当前变更

- 生命周期读取当前工作区的 `aiw.yaml`。本次验收覆盖未提交的新仓与业务基线、Change 分支提交后的脚本，以及只有 `origin/main` 的根仓。已有业务工作树仍使用创建时的基线 SHA。
- 已删仓库按本机状态中的 Store 校验归属，cleanup 优先读取当前声明，已删仓回退到状态快照。已有 `cleanup` JSON 状态键照旧，新增 `cleanupCommand` 和 `cleanupRecorded` 记录命令与快照存在性。旧状态缺少快照时，按 HEAD、远端跟踪基线、本地基线查询历史定义。保留工作树归属、远端和脏文件检查。
- Cobra 统一命令校验、全局选项与四种 shell 补全。`Run` 兼容原调用，`RunWithIO` 可注入 I/O。`-C` 同时用于命令路径及相对 `AIW_HOME`。Git 支持信号上下文、非交互凭据行为与脱敏计时，gofrs/flock 提示锁竞争并支持取消。YAML 拒绝陌生字段，保留 metadata 扩展，原子写入保留普通文件权限。
- Lipgloss 和 Huh 提供表格、选仓、释放确认和仓库操作进度。脚本直接透传 I/O。`doctor --json` 诊断环境、项目配置、登记和孤儿 Change，`change list --json` 按分支排序并标注缺失根目录。保留 `repo list --json` 与 `change status --json` 的数据结构。
- 事前设计见 [CLI 现代化与生命周期修复](docs/2026-11-20-cli-modernization-and-lifecycle-fix.md)。README、开发指南、AGENTS.md 和两份内置 skill 已同步，两份 skill 的 `AIW-GO-WORKTREE-V1` 标记均已回读。

## 验证

- `/usr/bin/time -p go test -race -timeout 5m ./...` 通过，`internal/aiw` 69.798 秒，端到端 75.07 秒。上一轮交接中的 235.477 秒来自旧测试集的非 race 运行。本轮增加了用例及 race 检测，并在 fixture 中隔离宿主 Git 配置、禁用提交签名和 fsmonitor。
- `go vet ./...` 与 `git diff --check` 退出 0，均无诊断。Go 1.26.5 运行 `scripts/build-release.sh` 生成 darwin/arm64、darwin/amd64、linux/arm64、linux/amd64，文件格式及 CPU 架构已读回。Go 1.22.12 另以 macOS 外部链接构建并执行 `--version`，输出 `dev`。
- 由 darwin/arm64 构建产物生成的 zsh 和 bash 脚本分别通过 `zsh -n` 和 `bash -n`。fish 与 PowerShell 均生成非空脚本。Go 集成测试通过 `__complete` 验证动态候选。Bash 交互使用前需加载 bash-completion，本机无该前置依赖。
- 以本机构建产物和临时仓进行 PTY 实测。选仓显示描述且保留预选，CI 和 `--no-input` 禁止表单，颜色关闭时表格无 SGR 样式，慢注册显示进度，关闭按两类风险确认分支名后释放业务 worktree。脚本 stdout/stderr 保持对应，实际锁竞争可由 SIGINT 或 SIGTERM 中断。
- 变更留在工作区供 Lead 审查，Git HEAD 为 `d2c69781c5fb07b3f2969b42e5b7122fc5d280d1`，暂存区为空，宿主 `~/.local/bin/aiw` SHA-256 与开工时一致。

## 剩余事项

- 验收使用临时 Git fixture 和 darwin/arm64 实测，其余三个目标仅交叉编译。fish 和 PowerShell 未执行真实补全会话。
- Go 1.22 兼容的 Bubble Tea 在真实终端会做一次后台色能力查询。关闭颜色控制样式输出，管道和 JSON 的回归使用重定向输出验证。
