# AIW Handoff

更新时间：2026-09-20

## 当前变更

- 新增 `.github/workflows/ci.yml`：PR/master 执行测试、vet 和四平台构建；`v*` tag 在验证成功后创建同名 GitHub Release，上传版本化归档及 SHA-256 清单。Release job 显式传递 `--repo "$GITHUB_REPOSITORY"`，不依赖 checkout 中的 `.git`；tag 会通过 `AIW_VERSION` 将版本注入二进制。内部制品库镜像尚未配置，且本地验证不等于远端 Release 已发布。设计记录见 `docs/2026-09-20-release-pipeline.md`。
- `aiw init` 新增 Agent Repo 的 `.aiw/aiw-toolchain.env` 与 `.aiw/bootstrap-aiw.sh`：前者锁定创建二进制的版本并预留四平台校验和，后者只从 HTTPS 制品目录下载、校验、安装并回读 `aiw version`。未填制品地址/校验和或开发版 `dev` 会在下载前拒绝；同名文件不覆盖。`aiw version` 是新增只读命令。
- `aiw.yaml` 每仓新增可选的项目自定义 `metadata` 映射。AIW 只解析并在 `repo list --json` 返回它；Git 物化、setup/cleanup 和关闭仍只依据原有字段。旧配置保持兼容。设计记录见 `docs/2026-09-20-repo-metadata-extension.md`。
- `README.md` 和 `docs/development.md` 已说明 metadata 边界、输出和不存放密钥/本机路径的约束。
- 本机全局 `/Users/bytedance/.local/bin/aiw` 已更新为本次构建的 darwin/arm64 二进制，SHA-256 与 `dist/aiw-darwin-arm64` 一致；这不是远端发布。

## 验证

本次发布流水线与 bootstrap 变更已通过 YAML/内嵌 shell 和 `scripts/build-release.sh` 的 `bash -n`、`git diff --check`、`go vet ./...`、四平台构建；带 `AIW_VERSION=v0.0.0-test` 的 darwin/arm64 二进制回读该版本。新增 init/bootstrap/version 测试和外部 worktree 集成场景通过。此前 `internal/aiw` 的其余测试也已按独立/小组 `go test -run` 通过；`cmd/aiw` 无测试文件。`v0.1.0` 的首次 Release job 因缺失 Git checkout 而在 `gh release create` 失败，已修复为显式指定仓库；重发 tag 后，Release job 与四个平台归档、`checksums.txt` 均已在远端读回成功。现有营销 Agent Repo 的七仓 `repo list --json` 均返回 metadata 且登记正常；当前 Change 的根仓仍 `ready: true`，五个业务仓干净且就绪。

## 剩余事项

本次源码及营销 Agent Repo 的迁移均未提交或推送。公开仓库的 Go 版 skills 仍需独立发布并核对远端安装；旧工作流 skills 的公开发布状态不能用本地二进制验证代替。当前仓库原有未提交改动保留。
