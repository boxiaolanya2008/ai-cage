# mihomo External Controller

## 连接约束

- 只允许 `http://127.0.0.1:<port>` 或 `https://127.0.0.1:<port>`。
- 请求使用 `Authorization: Bearer <secret>`。
- WebSocket 使用相同的鉴权 Header。
- Secret 不记录到日志，不放入 URL 查询参数。

## 已使用接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/version` | 内核版本与就绪检查 |
| GET | `/configs` | 运行配置、模式与 TUN 状态 |
| PATCH | `/configs` | 修改代理模式 |
| GET | `/proxies` | 代理组与节点 |
| PUT | `/proxies/{name}` | 选择节点，请求体为 `{"name":"..."}` |
| GET | `/connections` | 当前连接汇总 |
| GET | `/traffic` | WebSocket 实时流量 |
| GET | `/logs` | WebSocket 日志 |

`204 No Content` 是成功响应，不应按缺少 JSON 判错。

## 配置重载

ai-cage sidecar 在启动前生成完整运行时配置，不通过 Controller 重载任意用户路径。需要重载时，路径必须位于固定应用数据目录。

官方文档说明，工作目录之外的路径需要 `SAFE_PATHS`；Windows 使用分号分隔。AI Cage 不设置过宽的 `SAFE_PATHS`。

## Verge 模式限制

Verge 模式调用查询、模式切换和节点选择接口。AI Cage 不调用 Verge 的重载或进程停止接口，也不直接改写 Verge 配置文件。Verge 后续保存配置可能覆盖动态修改，因此以其界面显示为最终状态。

Clash Verge Rev v2.5.10 的 External Controller 默认地址为 `127.0.0.1:9097`，但“外部控制器”开关默认关闭。缺失或空 Secret 时会使用公开已知的占位值 `set-your-secret`，不能提供有效隔离；需要在 Verge 设置中开启 Controller，并改用高强度随机 Secret。不要为了连接 ai-cage 把 Controller 绑定到局域网。

Verge 模式只允许发送 `{"mode":"..."}` 的 `PATCH /configs`。该修改仅影响当前运行时，Verge 后续应用配置可能覆盖它；ai-cage 不将其显示为持久设置。

## Verge 快照

`internal/verge` 只读取 Clash Verge 当前运行配置，并删除 Controller、Secret、入站端口、TUN、DNS 等托管字段。只保留 `proxies`、`proxy-providers`、`proxy-groups`、`rules` 和 `rule-providers`。快照保存为 `Kind=verge` 的本地订阅，重复导入会替换原快照，不会自动跟随 Verge 后续变化。

## 应用笼规则

ai-cage 生成内置 `AI-CAGE` 代理组。启用“仅应用笼流量走代理”时，mihomo 使用官方 `PROCESS-PATH` 规则匹配应用完整路径，并设置 `find-process-mode: always`。出口 IP 检测通过本地 mixed 端口请求外部 echo 服务，只反映当前出口，不证明 DNS 零泄漏。

## 参考

- [mihomo 官方 API](https://wiki.metacubex.one/api/)
- [mihomo 官方仓库](https://github.com/MetaCubeX/mihomo)

接口行为必须在实际使用的 mihomo 版本上复验，尤其是代理选择请求体与 WebSocket 字段。
