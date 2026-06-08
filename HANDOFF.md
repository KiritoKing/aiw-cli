# AIW Handoff

更新时间：2026-06-08

## 当前活动

搭建 GitHub Actions npm 发布流水线和 `master` / `develop` / manual alpha 分支模型。

## 本次已处理

- 新增统一版本准备脚本 `scripts/prepare-release.mjs`：
  - 从 `package.json` 和公网 npm registry 读取当前状态。
  - `stable` 默认 patch bump；如果 `package.json` 的 stable base 高于 npm latest，则发布该 base，用于手动 major/minor。
  - `beta` 自动发布 `X.Y.Z-beta.N`，`N` 按同一 base 下已发布 beta 自动递增。
  - `alpha` 手动发布 `X.Y.Z-alpha.N`，支持 `--base-version X.Y.Z`。
  - 支持 `--dry-run`，并写 GitHub Actions outputs。
- 新增 GitHub Actions：
  - `.github/workflows/check.yml`：PR 和 `master`/`develop` push 上跑 Node 18/20/24 的 `npm run check`。
  - `.github/workflows/release-latest.yml`：push `master` 自动发布 npm `latest`，也支持手动传 `base_version`。
  - `.github/workflows/release-beta.yml`：push `develop` 自动发布 npm `beta`。
  - `.github/workflows/release-alpha.yml`：手动发布 npm `alpha`，可选择 `source_ref` 和 `base_version`。
- 发布 workflow 会先校验 `NPM_TOKEN`；缺少 secret 时会在写版本提交和 tag 前失败，避免留下半成品 release commit/tag。
- 发布 workflow 通过 token preflight 后，会在 CI 内更新 `package.json`，创建 `chore(release): vX.Y.Z...` 提交，创建同名 Git tag，然后执行 `npm publish --provenance --access public --tag <tag>`。
- README 英文主文档和 `README.zh-CN.md` 已同步发布与分支模型。
- 新增历史记录 `docs/2026-06-08-github-release-ci.md`。
- `package.json` 的 `npm run check` 已纳入 `scripts/prepare-release.mjs` 语法检查；npm pack 白名单仍只发布用户需要的 `scripts/install-global.sh`，不会把发布辅助脚本打进包。

## 验证结果

- 公网 npm live check：`@chlrc/aiw` 当前 latest 已是 `0.1.1`。
- `node scripts/prepare-release.mjs --channel stable --dry-run` 计算为 `0.1.2`。
- `node scripts/prepare-release.mjs --channel beta --dry-run` 计算为 `0.1.2-beta.0`。
- `node scripts/prepare-release.mjs --channel alpha --base-version 0.2.0 --dry-run` 计算为 `0.2.0-alpha.0`。
- `npm run check` 通过；npm 仍会输出本机 npmrc 的 `always-auth` / `email` / `home` unknown config warning，但不影响检查。
- `go run github.com/rhysd/actionlint/cmd/actionlint@latest` 通过。
- `npm pack --dry-run --json --registry=https://registry.npmjs.org/` 通过，包内容为 25 个文件，版本 `@chlrc/aiw@0.1.1`。
- `git diff --check` 通过。

## 当前阻塞

- GitHub 仓库需要设置 repository secret `NPM_TOKEN`，该 token 必须有 `@chlrc/aiw` 的 npm publish 权限。
- CI 发布仍需要确认 GitHub Actions 使用的 `NPM_TOKEN` 可发布 `@chlrc/aiw`；公网 npm 当前已有 `0.1.1`。

## 后续建议

- 配置 `NPM_TOKEN` 后，push 到 `master` 会发布 stable；push 到 `develop` 会发布 beta；手动运行 `Release alpha` workflow 会发布 alpha。
- 下一次 `master` release 预期发布 `@chlrc/aiw@0.1.2`。
- 首次 `develop` release 预期发布 `@chlrc/aiw@0.1.2-beta.0`。
