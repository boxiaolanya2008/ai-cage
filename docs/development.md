# 开发指南

## 环境

- Windows 10/11 x64
- Go 1.26
- Node.js 24、npm 11
- Wails v3 CLI

如果 `wails3` 不在 PATH：

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
```

本机默认安装到 `D:\go\bin\wails3.exe`。

## 初始化

```bash
npm --prefix frontend install
go mod tidy
```

## 开发运行

```bash
D:/go/bin/wails3.exe dev
```

Wails 会生成前端绑定、启动 Vite 并运行桌面程序。不要手改 `frontend/bindings`；它由 Go 服务生成。

## 常用验证

```bash
gofmt -w .
go test ./...
npm --prefix frontend run build
D:/go/bin/wails3.exe build
```

## 独立内核准备

1. 从官方发布获取目标架构的 `mihomo.exe`。
2. 在设置中填写路径并计算 SHA-256。
3. 添加订阅并下载。
4. 先不使用 TUN 验证 Controller 与节点。
5. 确认 Verge TUN 关闭后，再测试 sidecar TUN。

项目不会静默下载内核或替换用户已有的 Clash Verge。
