# AI Cage 工程规范

## 项目定位

AI Cage 是 Windows 优先的代理工作台桌面端。Go/Wails 负责界面与业务流程，mihomo 负责代理、规则与 TUN。首版支持两种互斥运行档位：

- **Verge 连接模式**：连接用户现有 Clash Verge 的 External Controller，不启动、不停止、不重载 Verge 内核。
- **独立内核模式**：启动 ai-cage 自己的 mihomo sidecar；只有 TUN 启用时通过 UAC 启动提权 core。

AI Cage 可以只读导入 Clash Verge 当前配置快照，并用独立内核运行。`AI-CAGE` 是内置代理组；“仅应用笼流量走代理”会为 AI Cage 启动的应用注入 `PROCESS-PATH` 规则，其他程序保持直连。

应用页的“打开沙盒窗口”会创建独立原生宿主窗口，并把目标应用的主窗口嵌入其中。它是窗口托管，不是安全沙盒，也不解除应用自身的系统隔离。

前端包含“新手教程”页，用于向新用户说明订阅购买、代理组、节点选择和沙盒窗口。教程不得提供规避平台检测、封号或绕过风控的内容。

TUN 只接管网络流量，不解除应用沙箱或完整性限制。应用启动器只创建普通用户进程。

## 技术栈

- Go 1.26、Wails v3.1.0
- React、TypeScript、Vite
- `gopkg.in/yaml.v3` 处理 Clash 配置
- `github.com/coder/websocket` 接收流量与日志
- Windows Win32 API 实现提权、Job Object 与进程生命周期

## 目录边界

- `main.go`：程序入口，识别 `--core`，创建 Wails 应用与托盘。
- `internal/app`：Wails 服务与前端绑定，只编排业务，不直接实现 Win32 细节。
- `internal/config`：设置与应用列表的持久化。
- `internal/subscription`：订阅下载、限制、YAML 校验与缓存。
- `internal/mihomo`：配置合并、Controller 客户端与 sidecar 进程。
- `internal/core`：提权 supervisor、loopback IPC 与生命周期。
- `internal/verge`：只读导入 Clash Verge 当前配置快照并清理托管字段。
- `internal/launcher`：普通应用启动。
- `internal/model`：跨层共享数据结构。
- `frontend`：中文桌面界面，不访问远程内容。

## 开发命令

在仓库根目录执行：

```bash
go test ./...
npm --prefix frontend run build
D:/go/bin/wails3.exe build
```

开发时使用 `D:/go/bin/wails3.exe dev`。如果 `wails3` 已加入 PATH，可省略绝对路径。

## 核心约束

1. **先读再改**：修改前读取相关模块、测试和 `docs/` 中对应文档。
2. **最小修改**：优先扩展现有包，不为了层数创建无价值抽象。
3. **Controller 仅 loopback**：禁止绑定 `0.0.0.0` 或局域网；所有请求带 Bearer Token。
4. **秘密不进日志**：Secret、订阅 URL 与 Token 不写入日志、错误信息或命令行参数。
5. **Verge 模式只读生命周期**：可以查询状态、切换模式和节点；不得替 Verge 启停内核、改 TUN 或重载其配置。
6. **并存必须互斥 TUN**：独立内核启用 TUN 前提示用户关闭 Verge TUN；无法自动确认时要求用户手动确认。
7. **应用笼规则**：`AI-CAGE` 是唯一内置出口组；`PROCESS-PATH` 规则只允许来自应用列表和 ai-cage 自身，不得注入任意路径。
7. **提权边界**：主程序清单保持 `asInvoker`。只提权 core/mihomo，不把 WebView 或普通启动应用提升为管理员。
8. **配置路径固定**：只允许应用数据目录下的配置，不开放任意路径重载，不设置过宽的 `SAFE_PATHS`。
9. **订阅输入不可信**：限制 HTTPS/HTTP、超时、响应大小和重定向；YAML 校验失败不得覆盖当前可用配置。
10. **失败回退**：sidecar 启动失败时明确报告并停止；不得留下无法追踪的网卡、路由或子进程。
11. **文档同步**：接口、权限、默认值或用户操作变化时，同步更新 README 与相关 `docs/`。
12. **不自动提交**：未经用户明确授权，不提交、不推送、不创建 PR、不合并代码。

## 测试要求

- 每个纯逻辑包至少覆盖正常路径与关键失败路径。
- Controller 测试使用 `httptest`，覆盖 Bearer 鉴权、204、错误响应、路径转义和 WebSocket 取消。
- 配置测试必须证明订阅的 proxies/groups/rules 被保留，Controller、Secret 与 TUN 覆盖符合预期。
- TUN、UAC、Wintun、DNS、睡眠恢复和 Verge 并存需要 Windows 真机验证；未执行时如实标注。
- 提交前运行 `gofmt`、`go test ./...`、前端构建与 Wails 构建。

## Skills

- `mihomo-integration`：修改 Controller、配置合并、订阅或内核生命周期时使用。
- `windows-release-testing`：修改提权、TUN、Job Object、托盘或发布流程时使用。

读取 Skill 后必须继续读取其引用的文档，不能只看 `SKILL.md`。

## 安全与边界

本项目是本地代理管理工具。不要加入远程命令执行、任意脚本执行、静默下载内核、自动绕过企业网络策略或伪装权限检查等功能。无法确认的操作先停止并说明，不猜测成功。
