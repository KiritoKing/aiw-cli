# GitHub CI 发布流水线与分支模型

日期：2026-06-08

## 目标

为 `@chlrc/aiw` 建立 GitHub Actions 发布流水线，并把稳定版、beta 版、alpha 版的分支与触发方式固定下来。

## 分支模型

- `master`：稳定版本线。push 到 `master` 自动发布 npm `latest`。
- `develop`：beta 版本线。push 到 `develop` 自动发布 npm `beta`。
- 任意分支：alpha 只通过 `Release alpha` 手动 workflow 发布，不自动发布。

## 版本规则

### stable

`master` 默认发布 patch 版本。CI 会读取公网 npm registry 上已发布的稳定版本：

- 如果当前 `package.json` 的 stable base 高于 npm latest，则发布当前 base。
- 否则基于 npm latest 自动递增 patch。

因此 major/minor 发布需要先手动 bump `package.json`，再合入 `master`。

### beta

`develop` 自动发布 `X.Y.Z-beta.N`：

- `X.Y.Z` 优先使用当前 `package.json` 中高于 npm latest 的 stable base。
- 否则使用 npm latest 的下一个 patch。
- `N` 根据同一 `X.Y.Z` 下已经发布过的 `beta.N` 自动递增。

### alpha

alpha 只手动发布，格式为 `X.Y.Z-alpha.N`。运行 `Release alpha` workflow 时可以选择源分支或填写 `source_ref`，也可以用 `base_version` 指定稳定 base。

## CI 文件

- `.github/workflows/check.yml`：PR 和 `master`/`develop` push 的 Node 18/20/24 检查。
- `.github/workflows/release-latest.yml`：`master` stable 发布。
- `.github/workflows/release-beta.yml`：`develop` beta 发布。
- `.github/workflows/release-alpha.yml`：手动 alpha 发布。
- `scripts/prepare-release.mjs`：统一版本计算与 `package.json` 更新。

## GitHub 配置要求

需要在仓库配置 repository secret：

```text
NPM_TOKEN
```

该 token 需要具备 `@chlrc/aiw` 的 npm publish 权限。

release workflow 会先校验 `NPM_TOKEN`，缺少 secret 时会在写版本提交或 tag 之前失败，避免留下未发布的 release commit/tag。

workflow 使用 npm provenance，所以 release job 需要：

```yaml
permissions:
  contents: write
  id-token: write
```

## 本地验证命令

```bash
node scripts/prepare-release.mjs --channel stable --dry-run
node scripts/prepare-release.mjs --channel beta --dry-run
node scripts/prepare-release.mjs --channel alpha --base-version 0.2.0 --dry-run
npm run check
```

2026-06-08 当前公网 npm 查询结果：`@chlrc/aiw` latest 已是 `0.1.1`，当前 checkout 也是 `0.1.1`。因此下一次 stable 自动发布会发布 `0.1.2`；首个 develop beta 会发布 `0.1.2-beta.0`。
