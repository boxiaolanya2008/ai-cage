# AI Cage

AI Cage 是 Windows 优先的本地代理工作台。它可以连接现有 Clash Verge 的 mihomo 内核，也可以运行独立的 mihomo sidecar，并通过可选的 TUN 模式接管普通桌面应用的网络流量。

> 状态：MVP 开发中。仓库已包含应用架构与自动化测试；Clash Verge 实机连接、Wintun、DNS、UDP 和睡眠恢复仍需在目标 Windows 环境验证。

## 运行模式

- **Clash Verge 模式**：查询现有 External Controller、查看流量、切换代理模式和节点。AI Cage 不启动、不停止、不重载 Verge 内核。
- **独立内核模式**：只读导入 Clash Verge 当前配置快照，并启动独立的官方 `mihomo.exe`。内置 `AI-CAGE` 代理组选择出口节点，可选进程规则只让 AI Cage 启动的应用走该节点。

两种模式互斥。不要在 Verge TUN 已开启时同时启用 AI Cage sidecar TUN。

## 架构

- Wails v3 + React + TypeScript 桌面界面与系统托盘
- Go 服务负责设置、Verge 快照导入、订阅、应用列表和 mihomo API
- 主界面不提权；只有 sidecar TUN 需要提权 core
- Controller 与 core IPC 仅绑定 loopback，并使用 Bearer Token
- 通过 Job Object 管理 sidecar 进程树
- 内置 `AI-CAGE` 代理组与应用笼 `PROCESS-PATH` 规则
- 独立沙盒宿主窗口：应用在 AI Cage 专用窗口中打开，而不是直接铺在桌面
- 新手教程页：说明订阅购买、代理组、节点选择与沙盒窗口
- 可选通过当前节点检测公网出口 IP

详见[架构说明](docs/architecture.md)、[mihomo API](docs/mihomo-api.md)和[Windows TUN](docs/windows-tun.md)。

## 环境要求

- Windows 10/11 x64
- 开发需要 Go 1.26、Node.js 24
- 桌面界面需要 WebView2 Runtime
- 独立内核模式需要官方 mihomo 可执行文件
- Verge 模式需要 Clash Verge 开启 External Controller（默认地址通常为 `127.0.0.1:9097`，默认可能关闭）

## 项目结构

```text
main.go                 程序入口、Wails 应用与托盘
internal/app            Wails 服务与业务编排
internal/config         设置与应用列表持久化
internal/subscription   订阅下载、限制、校验与缓存
internal/mihomo         配置合并、Controller 客户端、sidecar 进程
internal/core           提权 supervisor、loopback IPC 与生命周期
internal/verge          只读导入 Clash Verge 快照
internal/launcher       普通应用启动
internal/sandbox        嵌入应用窗口的原生宿主窗口
internal/model          跨层共享数据结构
frontend                React 界面；绑定自动生成，不手改
docs                    架构、API、TUN、开发、测试与发布文档
```

## 开发

```bash
go test ./...
npm --prefix frontend install
npm --prefix frontend run dev
D:/go/bin/wails3.exe dev
```

生产构建：

```bash
npm --prefix frontend run build
D:/go/bin/wails3.exe build
```

产物位于 `bin/ai-cage.exe`。

## 配置

运行数据保存在 `%APPDATA%\ai-cage`。sidecar 的 Controller Secret 自动生成，不通过命令行传递。Verge 模式请在设置中填写 loopback Controller 地址与 Secret。

## 安全边界

TUN 只接管网络，不会解除应用沙箱或完整性限制。应用启动器只创建普通用户进程。AI Cage 不静默下载 mihomo，不修改企业网络策略，也不把 Controller 暴露到局域网。

## 文档

- [开发指南](docs/development.md)
- [前端与界面约定](docs/frontend.md)
- [测试指南](docs/testing.md)
- [发布指南](docs/release.md)
- [English](README.md)

## 许可证

GPL-3.0-or-later，见 [LICENSE](LICENSE)。
