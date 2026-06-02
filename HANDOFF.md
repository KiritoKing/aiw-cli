# AIW Handoff

更新时间：2026-06-02

## 当前活动

为 AIW 新增 npm `skills` CLI 可消费的 agent skills，当前改动尚未提交。

## 本次已处理

- 新增 multi-skill 目录：
  - `skills/aiw-init/SKILL.md`
  - `skills/aiw-init/agents/openai.yaml`
  - `skills/aiw-reference/SKILL.md`
  - `skills/aiw-reference/agents/openai.yaml`
- `aiw-init` 用于帮助用户通过 npm 包和 `npx @chlrc/aiw init` 初始化 AIW 环境、做 dry-run、解释依赖门禁、选择 cmux scope 和排查 setup blocker。
- `aiw-reference` 用于帮助 agent 直接执行 AIW 日常操作，包括 workspace open、done、remove、gc、diff、git、commit，并强调高影响操作前先看真实状态和 dry-run/preview。
- 已更新 `package.json` 的 `files`，把 `skills` 目录纳入 npm 包发布内容。
- 已更新 `README.md` 和 `README.zh-CN.md`：
  - 用户路径改为 npm 包优先，不要求先 clone repo。
  - 首次使用可走 `npx @chlrc/aiw ...`。
  - 高频使用或深入定制时，再建议安装包或拉本地 checkout。
  - skills 安装示例改为从 `@chlrc/aiw` 包源安装；本地 `.` 只作为维护者发布前 discoverability 验证入口。

## 验证结果

- `npm run check` 通过。
- `npm_config_cache=/private/tmp/aiw-npm-cache npm pack --dry-run` 通过；dry-run 输出确认 npm tarball 包含：
  - `skills/aiw-init/SKILL.md`
  - `skills/aiw-init/agents/openai.yaml`
  - `skills/aiw-reference/SKILL.md`
  - `skills/aiw-reference/agents/openai.yaml`
- 使用 Ruby YAML 解析验证：
  - `skills/aiw-init/SKILL.md` frontmatter 可解析，name 为 `aiw-init`。
  - `skills/aiw-reference/SKILL.md` frontmatter 可解析，name 为 `aiw-reference`。
  - 两个 `agents/openai.yaml` 均可解析。

## 验证阻塞

- `python3 /Users/bytedance/.codex/skills/.system/skill-creator/scripts/quick_validate.py ...` 未能运行，原因是当前 Python 环境缺少 `yaml`/PyYAML。
- `npx --yes skills add . --list -y` 未完成，原因是当前 npm registry 指向 `https://bnpm.byted.org/skills` 且解析失败：`getaddrinfo ENOTFOUND bnpm.byted.org`。

## 后续建议

- 发布前用可访问 npm registry 和可写 npm cache 重跑：

```bash
npx --yes skills add @chlrc/aiw --list -y
npx --yes skills add @chlrc/aiw --skill aiw-init -y
npx --yes skills add @chlrc/aiw --skill aiw-reference -y
```

- 如果需要继续增强 skill，可补充更具体的命令 transcript 示例，但当前 `SKILL.md` 已保持自包含，没有依赖 repo README 才能使用。
