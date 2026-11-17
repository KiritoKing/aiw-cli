# AIW Handoff

更新时间：2026-11-17

## 当前变更

- 默认生成 Agent Repo 根目录的 `aiw.code-workspace`：根仓 `.` 在前，当前 worktree 的 `aiw.yaml` 全部业务仓按名称排序，使用 `repos/<name>` 相对路径，不按 Change 选择过滤，不创建或登记业务仓。
- `init`、`materialize` 和 `repair` 的物化协调流程自动同步；`vscode.autoSync: false` 关闭自动更新并保留旧文件。`aiw workspace sync` 在根仓或其子目录显式同步，即使自动更新已关闭也可执行。
- 每次同步原子覆盖整份固定输出文件，旧 folders、settings、tasks 均不合并。其他名称的 workspace 文件不变；符号链接和非普通目标拒绝覆盖。
- 生成文件通过本机 Git `info/exclude` 精确忽略，共用锁保护追加并保留原内容，不修改已提交的 `.gitignore`。已被 Git 跟踪的同名文件仍会被跟踪。
- 自动同步位于业务 worktree 结构变更前；目标路径失败会明确报错，修复后可重试或关闭自动同步。业务 setup/cleanup、状态格式和就绪判断保持原有语义。
- 配置、CLI 帮助、bash/zsh 补全、README、当前架构文档和两份内置 skill 已同步。事前设计见 `docs/2026-11-17-vscode-workspace-sync.md`。

## 验证

- `go test -v -timeout 10m ./...` 完整通过，`internal/aiw` 耗时 235.477 秒，既有和新增用例均执行；覆盖配置默认/开关/非法值、init、手动增删和整份覆盖、无登记与选择状态、物化/repair、失败重试、Git 排除和危险路径保护。
- 保留此前未完成的结果：首轮 `go test ./...` 在 233.234 秒时被主动 SIGQUIT 中止以取诊断栈，当时位于临时仓 `git commit` 退出后的 `exec.Cmd.awaitGoroutines`；随后 `go test -timeout 90s ./...` 因整套超时失败，当前测试 `TestSetupRunsOnEveryMaterializeAndExplicitRetry` 仅运行 14 秒。不能把这两轮记为通过；未据此修改 Git 实现、删减用例或绕过断言，最终使用原有 10 分钟上限完成全套。
- `go vet ./...`、`scripts/build-release.sh` 四平台构建、bash/zsh 补全语法检查和 `git diff --check` 通过。
- 按用户的新版本覆盖要求，PATH 当前命中的本机 `aiw` 已原子替换为本次 darwin/arm64 构建，回读版本 `dev`、帮助中的 `workspace sync` 和文件 SHA-256；与 `dist/aiw-darwin-arm64` 一致：`8f4c1369608d4dfaf443a5bca6bec754c8bda7e182c09a332f85e922d1757171`。
- 使用安装后的可执行文件在临时 Git 仓验证：默认 init 生成、从子目录读取当前配置、关闭自动更新后的显式同步、移除旧目录/settings/tasks、Git 忽略，以及不创建业务目录或登记状态。测试临时目录已清理。
- SSH 开发机 `cjadk-go-dev` 的既有 `~/.local/bin/aiw` 已更新为同次构建的 Linux/amd64 版本，回读版本 `dev`、帮助和 SHA-256；与 `dist/aiw-linux-amd64` 一致：`46abee53c03d04a66e1df9eed7f802dc212f91dddf75c508ddfef265a9349835`。使用远端安装后的 CLI 在临时 Git 仓实测默认生成、子目录手动同步、关闭自动更新后的显式同步、整份覆盖、Git 忽略，以及不创建业务仓或登记状态；临时产物已清理。安装仅替换已有二进制，未改远端 shell 配置或重启服务。

## 剩余事项

Git 交付分支为 `snobbish-anaconda`，远端为 `origin`；未发布远端 Release。未在真实业务 Agent Repo 中执行物化或同步，未做 VS Code GUI 验收；当前证据为源码、临时 Git 集成测试、构建、本机及 SSH 开发机安装后 CLI 实测。未启动服务，未修改真实业务 checkout 或全局 Git 配置。
