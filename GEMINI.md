# Gemini 工作规范（AI Cage）

本文件只记录 Gemini CLI 场景下的补充规则。项目通用规则见 [AGENTS.md](AGENTS.md)。

## 上下文读取

- 先用目录与行数确认文件规模，再有选择地读取。不要把 `internal/app/service.go`、`internal/subscription/manager.go`、`frontend/src/App.tsx` 整份读入后再找重点，先定位函数与测试。
- 关注跨文件调用链：`internal/app` 编排 → `internal/launcher` / `internal/mihomo` / `internal/core` → Win32 层。改一处前确认调用方与测试。
- 前端类型来自 `frontend/bindings`，该目录由 Wails 生成，不手改。`frontend/src/types.ts` 是再导出的适配层。

## 工具调用

- 搜索优先用 `rg` 限定目录和文件类型，例如 `rg -n "PROCESS-PATH" internal`。
- 修改文件使用精确替换，不做大段重写；一次只改一个逻辑单元。
- 命令行验证按项目约定执行，不新建构建脚本。

## 多模态与配置

- 截图、界面图片只能用于理解现状，不能据此推断未实现的功能。
- 涉及 YAML 配置时，以 `internal/mihomo/config.go` 的合并逻辑为准，不依据通用 Clash 文档猜测字段。

## 验证与报告

- 修改 Go 代码后运行 `gofmt -l .`、`go test ./...`。修改前端后运行 `npm --prefix frontend run build`。
- 报告只写实际执行过的检查；未执行的真机场景在 `docs/testing.md` 中补记时注明环境与版本。