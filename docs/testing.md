# 测试指南

## 自动化测试

```bash
go test ./...
npm --prefix frontend run build
D:/go/bin/wails3.exe build
```

自动化测试覆盖：

- 设置加载、默认值与损坏文件
- 订阅 URL、超时、大小和 YAML 校验
- mihomo 配置合并与敏感字段覆盖
- Controller Bearer 鉴权、错误响应和路径转义
- WebSocket 流量、日志与上下文取消
- core IPC 鉴权、mihomo 哈希和状态转换
- 普通应用启动参数

## Windows 真机矩阵

| 场景 | 验证点 |
|---|---|
| Verge 普通连接 | 版本、模式、节点、流量、日志 |
| Sidecar 普通连接 | 独立端口、订阅、启动与停止 |
| Sidecar TUN | Wintun、默认路由、TCP/UDP/DNS |
| 权限 | UI 普通权限，core 提权，启动器不提权 |
| 并存 | Verge 与 sidecar 不同时启用 TUN |
| 异常 | 拒绝 UAC、哈希错误、配置损坏、内核退出 |
| 恢复 | 睡眠唤醒、断网、Wi-Fi 切换、重复启停 |

## 报告要求

真机报告记录 Windows 版本、ai-cage 提交、mihomo/Verge 版本、测试时间、实际结果和未覆盖项。没有在目标环境执行的测试不得标记为通过。

## 2026-11-26 本机验证记录

- Windows 10.0.26200 x64。
- `go test ./...` 与 `go vet ./...` 通过。
- `npm --prefix frontend run build` 通过。
- Wails v3.1.0 生产构建通过，产物为 `bin/ai-cage.exe`。
- 使用本机 Clash Verge 附带的 `verge-mihomo.exe` v1.19.33 完成非 TUN sidecar 实测：配置校验、SHA-256、core 鉴权、启动、`/version`、`/configs`、`/proxies`、`/connections`、节点选择、模式切换、停止与清理均通过。
- 停止测试曾暴露 Windows `TerminateProcess: Access is denied` 误报，已修复并复测通过；sidecar 端口与 `core.json` 均确认清理。
- 本机 Clash Verge v2.5.10 正在运行，但 External Controller 开关关闭、9097 未监听，因此 Verge 真实连接、UAC、Wintun、TUN、DNS、UDP 与睡眠恢复尚未验证。

## 2026-11-26 应用笼与快照验证记录

- 已只读导入本机 Clash Verge 当前配置快照；AI Cage 自动切换为独立内核，启用 TUN 与“仅应用笼流量走代理”。
- 已配置本机 `verge-mihomo.exe` v1.19.33 及其 SHA-256。
- 自动化测试新增并通过：Verge 快照清理、当前 profile 回退、重复导入替换、`AI-CAGE` 组生成、`PROCESS-PATH` 规则顺序、`MATCH,DIRECT` 兜底。
- 前端重新生成 Wails bindings，TypeScript 与生产构建通过。
- 新版 Wails 桌面程序已启动，窗口标题正常。
- 修复 `SHELLEXECUTEINFOW` 结构体缺少 `hkeyClass` 导致的 `ShellExecuteEx: Access is denied`；提权启动专项测试通过。
- 修复 Verge 模式下 AI Cage TUN 开关误显示为开启的问题；切换到独立内核时 TUN 自动开启，切换回 Verge 时自动关闭。
- 本机 Clash Verge 当前 TUN 为开启状态，因此未启动 AI Cage TUN，避免双重 TUN。用户需先在 Verge 中关闭 TUN，再启动 AI Cage 并进行真实应用笼流量验证。
- 后续只读检查发现导入快照包含 `10.255.255.255` 信息占位节点，且原规则已有最终 `MATCH`，导致“其他程序直连”兜底不可达。已修复：导入时清理占位节点，合并时替换最终 `MATCH` 为 `MATCH,DIRECT`，并让 `ipv6` 跟随 AI Cage 设置。
- 新增独立沙盒宿主窗口：应用页按钮改为“打开沙盒窗口”，目标应用会在 AI Cage 专用原生窗口中嵌入显示。
- 沙盒窗口是窗口托管，不是 Windows Sandbox，也不提供安全隔离；部分多进程或多窗口应用可能无法稳定嵌入。
- 已改进窗口匹配：启动前记录现有顶层窗口，启动后优先找新增窗口，并按进程映像名兜底，改善 Edge/Chrome 这类复用已有主进程的应用。

