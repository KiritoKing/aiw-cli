# AIW 可复现发布流水线

## 现状

`scripts/build-release.sh` 可在本地交叉编译 macOS 和 Linux 的 arm64、amd64 裸二进制，但仓库没有 CI 配置，也没有可供干净开发容器校验和下载的版本化制品。

## 目标

在 GitHub Actions 中提供两个受同一工作流管理的门禁：

- `push` 到 `master` 和 pull request 均执行 `go test ./...`、`go vet ./...` 及四平台交叉编译；
- 推送形如 `v*` 的 Git tag 时，在上述验证成功后，将四个平台的压缩包和 `checksums.txt` 发布到同名 GitHub Release。

压缩包名为 `aiw_<version>_<os>_<arch>.tar.gz`，内部开发容器可将该 Release 同步或镜像到内部制品库，并按仓库锁定的版本及 SHA-256 安装。CI 不持有内部制品库凭据，也不在此变更中执行镜像同步。

`aiw init` 会在新 Agent Repo 写入 `.aiw/aiw-toolchain.env` 和 `.aiw/bootstrap-aiw.sh`。前者锁定创建该 Agent Repo 的 AIW 二进制版本，并为四个平台保留独立 SHA-256 字段；后者只读取这一锁文件，从团队指定的 HTTPS 制品目录下载对应归档、校验、安装并以 `aiw version` 回读版本。开发构建的版本是 `dev`，或任何缺失制品地址/校验和的锁文件，都会在联网前失败，必须由团队用已发布 Release 的版本、镜像地址和校验和更新后提交。

## 影响范围与兼容性

- 新增只读 `aiw version` 命令；发布构建用 Git tag 注入该值，未注入的本地构建保持 `dev`。
- 不改变 AIW 的配置格式、worktree 生命周期或本机数据。
- 既有 `scripts/build-release.sh` 仍只负责本地构建裸二进制；Release 打包仅在 CI 的发布 job 中进行。
- tag 发布是显式外部可见写入，只有 GitHub tag 触发才执行；分支和 PR 运行仅验证和上传短期 Actions artifact。

## 数据与安全边界

- 工作流只使用仓库默认的 `GITHUB_TOKEN`，并仅在发布 job 申请 `contents: write` 权限。
- 下载方必须使用 `checksums.txt` 校验归档；不提供动态 `latest` 安装脚本，也不使用 `curl | sh`。
- 归档由 tag 对应的不可变源码构建。GitHub Release 之外的内部镜像、权限和保留策略由内部制品平台负责。
- bootstrap 不在 `init` 时执行，也不接受动态 `latest`、不校验的 checksum URL 或凭据。它只在显式运行时覆盖目标 `aiw` 二进制，成功标准是哈希和二进制自身版本均与锁文件一致。

## 验收

1. workflow YAML 语法可解析，shell 打包片段通过 `bash -n`。
2. 本地 `go test ./...`、`go vet ./...` 和 `scripts/build-release.sh` 通过。
3. 在 GitHub 上，PR/master run 可下载四个平台的构建 artifact；`v*` tag run 成功后，同名 Release 包含四个 `.tar.gz` 与 `checksums.txt`。
4. `aiw init` 生成版本锁和可执行 bootstrap；未填写地址、校验和或版本为 `dev` 时 bootstrap 在下载前报错。
