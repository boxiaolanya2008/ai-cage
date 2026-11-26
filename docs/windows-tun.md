# Windows TUN 与提权

## 能力边界

TUN 从虚拟网卡层接管 TCP/UDP，不依赖应用是否使用系统代理。但它不解除 AppContainer、低完整性令牌或 Windows Sandbox 等隔离。ai-cage 的应用启动器只启动普通进程。

## 权限模型

- Wails UI 始终以普通权限运行。
- Sidecar 普通代理模式不需要提权。
- Sidecar TUN 通过 UAC 启动 core，再由 core 启动 mihomo。
- core 使用 Job Object 管理 mihomo 进程树。
- 用户拒绝 UAC 时取消启动，不重复弹窗。

当前开发构建可以直接从工作目录运行，但正式启用 TUN 前应把 UI/core 安装到受保护目录（例如 `Program Files`）并验证 Authenticode 签名。不要从用户可随意替换的便携目录提权执行 core。

## 默认配置

- `stack: gvisor`
- `device: ai-cage`
- `auto-route: true`
- `strict-route: true`
- `auto-detect-interface: true`
- `dns-hijack: any:53`
- Fake-IP 网段：`198.18.0.1/16`
- 默认上游 DNS：`223.5.5.5`、`119.29.29.29`，可在设置中修改

启用“仅应用笼流量走代理”时，ai-cage 会：

1. 设置 `find-process-mode: always`。
2. 为应用列表中的完整 `.exe` 路径注入 `PROCESS-PATH,<path>,AI-CAGE`。
3. 自动加入 ai-cage 自身路径，用于出口 IP 检测。
4. 在原有规则前插入进程规则，并追加 `MATCH,DIRECT`，让未加入应用笼的程序保持直连。

关闭该选项时，ai-cage 保留订阅原有规则，由 TUN 接管全部流量。

DNS 上游是否经过代理取决于具体 mihomo 规则和配置。不能仅凭开启 TUN 就声称 DNS 或所有 UDP 流量一定经过代理。

## 与 Clash Verge 共存

ai-cage 不会自动关闭 Verge TUN。启用 sidecar TUN 前，用户必须确认 Verge TUN 已关闭。两个 TUN 虚拟网卡同时修改默认路由可能导致断网、DNS 泄漏或规则竞争。

Sidecar 使用独立数据目录和 GeoIP/GeoSite 文件，不与 Verge 共享可写缓存。配置合并会禁用订阅携带的 HTTP、SOCKS、redir 和 TProxy 入站端口，只保留 ai-cage 自己的 mixed 端口，避免与 Verge 默认端口冲突。

## 必须真机验证

1. Wintun 驱动安装与卸载后的残留。
2. TCP、UDP、DNS、IPv6 的流量路径。
3. `PROCESS-PATH` 在目标应用上的匹配结果，包括子进程和提权应用。
4. 启动失败时路由和虚拟网卡是否清理。
5. 睡眠、唤醒、Wi-Fi 切换和 VPN 叠加后的恢复。
6. 非管理员 UI 停止提权 core 是否可靠。
7. Verge 与 sidecar 切换时是否出现双重 TUN。

任一项失败都应停止启用并报告，不把半启用状态显示为成功。
