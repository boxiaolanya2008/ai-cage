# Claude 工作规范（AI Cage）

本文件只补充 Claude Code 的使用方式。项目通用规则以 [AGENTS.md](AGENTS.md) 为准，两者冲突时以 `AGENTS.md` 的安全与边界条款为准。

## 工作方式

- 直接在当前会话完成修改。不要为了并行而批量派生子 Agent；只有用户明确要求并行调研时才使用，且子 Agent 只读。
- 进入实现前先读 `AGENTS.md`、相关 `docs/` 和目标包的测试文件。不要凭文件名猜架构。
- 需要长任务时分成“读取 → 最小修改 → 验证”三步，每步结束前不要提前宣布完成。

## Skills

- 修改 Controller、订阅、配置合并、内核生命周期前，读取 `.agents/skills/mihomo-integration/SKILL.md`。
- 修改提权、TUN、Job Object、托盘、构建清单或发布前，读取 `.agents/skills/windows-release-testing/SKILL.md`。
- 修改 `frontend/` 前，读取 `.agents/skills/frontend-ui/SKILL.md`。
- 读完 `SKILL.md` 后必须继续读取它列出的 References 文件，不能只读入口。

## 计划与确认

- 可以用计划模式梳理方案，但计划必须落到具体文件与验证命令，不以“看起来可行”结束。
- 涉及 UAC、TUN、进程树、删除用户数据、覆盖现有配置的操作，先说明后果再执行。
- 不代替用户提交、推送、合并，除非用户当次明确要求。

## 验证

- 任何“已修复”的结论都要有命令输出支撑：`go test ./...`、`go vet ./...`、`npm --prefix frontend run build`、`D:/go/bin/wails3.exe build`。
- 真机未验证的 Windows 行为（Wintun、DNS、UDP、睡眠恢复）必须标注“未验证”，不许用构建成功代替。