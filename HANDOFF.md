# AIW Handoff

更新时间：2026-09-20

## 当前活动

已实现根仓 `AGENTS.md` 的 AIW 托管区块：`init` 创建或更新区块，`aiw agents sync` 在已有 Agent Repo 只替换完整标记之间的内容，保留用户区块外字节；异常标记与符号链接会拒绝写入。`init` 默认调用社区 `npx skills add` 从公开 AIW 仓库安装两份项目级 skills，`--skip-skills` 可交由用户自己的安装器；没有 Node/npx 时仅用 Go 内置副本补齐缺失文件，已有不同内容不覆盖。新增 `repo list --json` 与 `change status --json` 的业务仓 `baseSha`，更新帮助、补全、README、开发文档、skills 和日期设计记录。

## 验证

`go test ./...`、`go vet ./...`、`scripts/build-release.sh` 和 `git diff --check` 通过。临时目录中实际运行社区 `skills` CLI，以本地源码作为来源，确认两份 skills 复制到 `.agents/skills`；用构建后的二进制在无 Node/npx 的 PATH 下运行 `init`，核对内置兜底文件。真实公开仓库安装也已测试：仓库仍提供旧 Node 工作流内容，AIW 的 Go 工作流标记检查按预期让 `init` 报错；已生成的 Agent Repo 与安装器文件保留，临时验证目录已清理。测试覆盖区块边界、同步幂等、错误标记、符号链接、跳过安装、Go 兜底与冲突、npx 参数和旧版检测、JSON 输出。

## 剩余事项

公开仓库须发布本版 Go skills 并再次核对远端安装，默认 `init` 才能完整成功。本轮未提交、推送或更新全局 AIW 安装。工作区原有的 Go 重构仍未提交；本轮保留了它及与本任务无关的改动。
