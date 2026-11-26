# 架构说明

## 目标

AI Cage 将桌面控制界面与 mihomo 内核分离。界面负责订阅、节点、流量和应用启动；内核负责代理、规则与 TUN。权限较高的操作限制在独立 core 进程中。

## 运行档位

```text
Wails UI（普通权限）
  ├─ Verge 模式 ── HTTP/WebSocket ──> Clash Verge 的 mihomo
  └─ Sidecar 模式 ── loopback IPC ──> core supervisor
                                      └─ mihomo.exe（TUN 时提权）
```

Verge 快照导入是独立的数据通道：

```text
Clash Verge 当前配置 ──只读──> internal/verge
                              └─ 清理托管字段
                                 └─ AI Cage 本地订阅快照
```

两种运行档位互斥：

- Verge 模式只读取和控制 External Controller，不管理 Verge 进程与配置文件。
- Sidecar 模式使用 ai-cage 的订阅缓存和运行时配置。
- 任一内核启用 TUN 时，另一个内核不应同时启用 TUN。

## 模块职责

| 模块 | 职责 |
|---|---|
| `internal/app` | 前端服务、模式切换、业务编排 |
| `internal/config` | 设置、应用列表和原子持久化 |
| `internal/subscription` | 订阅下载、大小限制、YAML 校验和缓存 |
| `internal/mihomo` | 配置合并、Controller 客户端、sidecar 启动 |
| `internal/core` | 提权入口、loopback IPC、状态与停止 |
| `internal/verge` | 只读导入当前 Verge 配置并清理托管字段 |
| `internal/launcher` | AI Cage 应用笼的普通应用启动 |
| `internal/sandbox` | 独立原生宿主窗口，把应用主窗口嵌入 AI Cage |
| `internal/model` | 跨层数据结构 |

## 数据流

1. UI 读取 `%APPDATA%\ai-cage\settings.json`。
2. Sidecar 模式选择已下载订阅或 Verge 快照，合并 ai-cage 的 Controller、Secret、TUN 与 DNS 覆盖项。
3. `internal/mihomo` 生成内置 `AI-CAGE` 代理组；启用“仅应用笼流量走代理”时，把应用列表和 ai-cage 自身的完整路径转换成 `PROCESS-PATH` 规则，并追加 `MATCH,DIRECT` 兜底。
4. UI 将固定路径的启动请求交给 core；请求不包含内核 Secret。
4. core 校验 mihomo 路径与 SHA-256，启动子进程并等待 `/version` 就绪。
5. UI 使用运行时生成的 loopback Controller Secret 查询状态、代理组、流量和日志。
6. 停止请求始终发送给 core，由持有 Job Object 的进程回收 mihomo 进程树。
7. 用户显式点击“检测出口 IP”时，AI Cage 通过本地 mixed 端口请求外部 echo 服务；因 ai-cage 自身也在进程规则内，结果反映 `AI-CAGE` 当前节点出口。

## 权限边界

主程序使用 `asInvoker` 清单。只有 sidecar TUN 模式通过 `runas` 启动 core。普通应用由非提权 UI 启动，不继承管理员权限。

Verge 模式的 TUN 与订阅文件仍由 Clash Verge 管理。AI-cage 不通过文件写入模拟 Verge 操作，避免配置竞争。
