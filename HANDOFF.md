# AIW Handoff

更新时间：2026-06-01

## 当前活动

开源前审计、GitHub 首次推送、npmjs 首次发布已完成。

## 本次已处理

- 将默认配置从个人机器路径改为可移植路径：
  - `config/aiw.toml` 的 `code_root` 改为 `~/Code`。
  - `config/aiw.toml` 的 `core_config` 改为 `~/.config/aiw`。
  - `src/config.mjs` 的缺省 `code_root` 改为当前用户 home 下的 `Code`。
- 将默认 commit agent 从内部/个人化工具改为公开配置里的 `codex`。
- 从默认 `config/agents.toml` 移除内部/个人化 agent adapter。
- 移除 `package.json` 的 `private: true`，避免公开仓库和 npm 元数据语义冲突。
- 新增 `.gitignore`，排除 `node_modules`、构建产物、日志、临时文件和 `.env*`。
- 将公开 README、中文 README、AGENTS 与历史文档里的示例路径改为 `~/Code` 或 `<code-root>` 形式。
- 清理当前 handoff 中的内部飞书文档链接和命令记录。
- npm 发布元数据已准备：
  - `package.json` 包名为 `@chlrc/aiw`。
  - `publishConfig.registry` 固定为 `https://registry.npmjs.org/`。
  - `publishConfig.access` 固定为 `public`。
  - README 中的初始化命令已改为 `npx @chlrc/aiw init`。
  - npm tarball 通过 dry-run 验证，包含 21 个文件，约 45.3KB。
- 已发布 `@chlrc/aiw@0.1.0` 到 npmjs，dist-tag 为 `latest`。
- 已从 npmjs 安装 published 包并验证 `aiw --help` 可运行。

## 审计结论

- 当前工作树内容未发现明显密钥特征、私钥块或常见云服务访问密钥格式。
- 当前公开文件中仍有工具名、工作流设计、cmux/Worktrunk/lazygit/delta 等产品上下文；这些属于功能说明，不是密钥。
- npm 包名已设置为 `@chlrc/aiw`，并通过 `publishConfig` 固定发布到 npmjs public registry；license 仍需后续明确。
- 发布目标已确认是 npmjs：`Publishing to https://registry.npmjs.org/ with tag latest and public access`，没有发布到内网 registry。

## 需要特别注意

- Git 历史已确认仍保留开源前的个人路径、内部文档链接和内部/个人化 agent 痕迹。首次推到公开 GitHub 时，不建议直接推完整历史；更稳妥的方式是从当前清理后的工作树生成干净的首提交再发布。
- 当前仓库还没有选择开源许可证；没有 LICENSE 时，GitHub 可以公开，但严格意义上不便于外部使用和贡献。

## 验证建议

开源前至少跑：

```bash
npm run check
git diff --check
node bin/aiw doctor --gate git
node bin/aiw doctor --gate commit --agent codex
node bin/aiw layout --agent codex --dry-run
```

首次推送前确认：

```bash
git status --short --branch
git remote -v
```

发布命令：

```bash
npm publish --access public --registry=https://registry.npmjs.org/ --cache /private/tmp/aiw-npm-cache
npm view @chlrc/aiw@latest version --registry=https://registry.npmjs.org/
```
