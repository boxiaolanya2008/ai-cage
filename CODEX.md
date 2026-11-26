# Codex 工作规范（AI Cage）

本文件是 Codex 场景的补充说明。通用工程规则仍以 [AGENTS.md](AGENTS.md) 为准。

## 代码库探索

- 先看 `AGENTS.md` 的目录边界，再进入目标包；不要跨层实现（例如在 `internal/app` 里写 Win32 调用）。
- 定位行为的顺序：`internal/model` 数据结构 → 对应包实现 → 该包测试 → `docs/` 说明。
- 需要确认“是否已经实现”时，以代码和测试为准，不以 README 措辞为准。

## 修改策略

- 优先扩展现有函数与结构体字段，不新增抽象层、不新建工具包。
- 保持既有错误处理风格：错误信息可读、不含 Secret 与订阅 URL。
- 新增导出方法后，用 Wails 重新生成绑定，不在 `frontend/bindings` 手写。

## 并行任务

- 仅在读取类任务上并行（例如同时检查多个包）。写入操作串行执行，避免同一文件并发修改。
- 不派生子 Agent 代为提交、推送或删除文件。

## 验证

- 提交前依次运行：`gofmt -w .`、`go test ./...`、`go vet ./...`、`npm --prefix frontend run build`、`D:/go/bin/wails3.exe build`。
- 失败时先读完整错误输出再改，不通过放宽断言或跳过测试来“修复”。

## Git

- 使用 Conventional Commits。当前仓库使用 SSH 签名，提交应保持可验证签名。
- 未经用户明确授权，不提交、不推送、不创建或合并 PR。