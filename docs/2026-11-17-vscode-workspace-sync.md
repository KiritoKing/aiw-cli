# VS Code 多仓工作区同步

## 现状与目标

AIW 将业务 worktree 挂载到 Agent Repo 的 `repos/<name>`，目前没有生成编辑器工作区的能力。为便于同时阅览根仓上下文与各业务仓代码，新增默认开启的 VS Code multi-root workspace 生成能力。

## 配置与命令契约

- 当前 worktree 的 `aiw.yaml` 是唯一输入；包含根仓 `.` 和 `repos` 声明的全部业务仓，业务仓按名称排序。这里使用固定仓库集合，不按 `.aiw/change.yaml` 筛选，也不读取注册来源的个人路径。
- 输出固定为 Agent Repo 根目录的 `aiw.code-workspace`，使用 VS Code 官方支持的 `folders[].name`、`folders[].path` 和相对路径。未物化的仓库仍保留目录项；生成不创建或登记业务仓。
- `vscode.autoSync` 为可选布尔值，省略或 `true` 表示自动同步；`false` 仅关闭自动同步，保留既有文件。
- `init` 和 `change materialize` / `change repair` 的结构协调流程自动生成或更新文件。物化协调在业务 worktree 结构变更前同步，自动同步失败必须明确返回错误；可修复目标路径后重试，或通过配置关闭自动同步，不回滚之前已完成的物化。
- `aiw workspace sync` 从当前目录向上查找 `aiw.yaml`，在 Agent Repo 根仓或 Change worktree 中都可执行；显式命令不受 `autoSync` 开关限制，不要求仓库登记、选择清单或 `AIW_HOME` 状态。
- 每次同步原子覆盖整个固定输出文件，不合并旧 folders、settings、tasks 等自定义内容；其他名称的 workspace 文件不变。不启动 VS Code 或开发服务器，不引入文件监听服务。

## 兼容、数据安全与失败恢复

- 旧版 `aiw.yaml` 无需迁移，默认开启。新字段不改变现有 version、repo、metadata、setup/cleanup、Change 状态格式及其 Git 语义。
- 生成文件仅含项目名称与相对目录，不写入 remote、metadata、凭据或本机绝对路径。
- 通过当前 Agent Repo 的本机 Git `info/exclude` 精确忽略 `/aiw.code-workspace`，避免自动生成导致旧项目根仓出现未跟踪文件；保护原有排除规则，不修改已提交的 `.gitignore`。Git exclude 的更新使用共用锁，避免不同 Change worktree 同时同步时丢失内容。
- 固定输出路径及需修改的 exclude 文件若为符号链接或非普通文件，拒绝覆盖。业务 worktree 与原始 checkout 不作任何额外写入。
- 输出写入失败时保留已有文件；关闭自动同步不删除既有文件。Change 生命周期和就绪判定继续使用原有状态与 setup 结果，workspace 文件不作为业务仓可用性证明。

## 验收方法

- 验证旧配置默认开启、显式开启/关闭和非法配置值，生成内容符合 multi-root folders 格式且排序稳定、仅使用相对路径。
- 验证手动同步增删仓库及整份覆盖、配置关闭后的自动跳过与显式同步、空仓库集合、从子目录执行、无登记/选择状态，以及错误参数在写入前被拒绝。
- 验证 init、materialize、repair 的自动入口及根仓 Git 忽略；验证文件写入失败、符号链接拒绝和 exclude 内容保护。
- 执行 `go test ./...`、`go vet ./...`、`scripts/build-release.sh` 和 shell 补全语法检查；测试使用临时 Git 仓，不修改真实业务仓。不把配置与 CLI 测试当作 VS Code GUI 实测。

VS Code 文件结构依据：[Multi-root Workspaces](https://code.visualstudio.com/docs/editing/workspaces/multi-root-workspaces#_workspace-file-schema)。
