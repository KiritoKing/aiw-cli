# AIW Handoff

更新时间：2026-06-02

## 当前活动

本机 AIW 全局配置已生成，且已为两个业务仓库配置 workspace 初始化安装脚本。

## 本次已处理

- 通过项目自带初始化流程生成全局配置目录：`/Users/bytedance/.config/aiw`。
- 已生成全局配置文件：
  - `/Users/bytedance/.config/aiw/aiw.toml`
  - `/Users/bytedance/.config/aiw/agents.toml`
  - `/Users/bytedance/.config/aiw/commit-prompt.md`
  - `/Users/bytedance/.config/aiw/lazygit-delta.yml`
- 初始化时显式跳过 cmux 注册：`--cmux-scope none`。
- 在全局 `aiw.toml` 写入项目级 `pre_init` hook：
  - `cjpay_promotion_lynx_next`: `emo i && emo run bam`
  - `marketing-x`: `emo i && npm run cgi-init && npm run pull:auto && npm run pre-build`

## 验证结果

- `node bin/aiw doctor --gate base --json` 已确认 AIW 从 `/Users/bytedance/.config/aiw` 读取配置。
- 在 `/Users/bytedance/Code/cjpay_promotion_lynx_next` 执行 `aiw layout --agent codex --dry-run`，确认命中全局项目 hook，且执行目录为该仓库根目录。
- 在 `/Users/bytedance/Code/marketing-x` 执行 `aiw layout --agent codex --dry-run`，确认命中全局项目 hook，且执行目录为该仓库根目录。

## 需要特别注意

- 这次没有向业务仓库写入 `.aiw.toml`，所有脚本都在本机全局配置中。
- `pre_init` 会在真实 `aiw layout` / `aiw cmux-new` 初始化 workspace 前执行；dry-run 只打印计划，不会安装依赖。
- `cmux-git-diff` 当前仍未安装，但 `delta` 可用，layout 依赖门禁可以通过。
