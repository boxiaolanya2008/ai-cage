# 前端与界面约定

## 技术栈

- React 18 + TypeScript，Vite 8 构建，输出到 `frontend/dist` 后由 `main.go` 的 `//go:embed all:frontend/dist` 打包进可执行文件。
- 不使用路由库，页面切换由 `frontend/src/App.tsx` 的 `page` 状态控制。
- 界面文案为中文；`eyebrow` 等装饰性标签保留英文。

## 文件职责

| 文件 | 职责 |
|---|---|
| `frontend/src/App.tsx` | 页面状态、业务调用、各页面组件（Dashboard、Subscriptions、Nodes、Apps、SettingsPage） |
| `frontend/src/components.tsx` | 图标（`Icon`）、`PageHeader`、`Modal`、`Toggle`、`Field`、`EmptyState`、`Toasts`、`BrandMark` |
| `frontend/src/tutorial.tsx` | 新手教程与合规提醒 |
| `frontend/src/types.ts` | 再导出 `frontend/bindings` 生成类型，并把 Go 的 `[]string` 字段适配为 TypeScript 数组 |
| `frontend/src/styles.css` | 全部样式；使用 CSS 变量，跟随 `prefers-color-scheme` 切换深浅色 |

## 绑定

- `frontend/bindings/` 由 Wails 根据 `internal/app` 导出的方法和 `internal/model` 结构生成，不在其中手改。
- Go 侧新增或修改导出方法后，重新生成绑定；`frontend/src/types.ts` 只做类型再导出，不重复定义数据结构。
- 前端通过 `AppService.<Method>` 调用，方法名与 Go 导出一致。

## 数据刷新

- 界面不用 Wails 事件推送，改为轮询：连接运行中时每 2 秒调用一次 `GetSnapshot`。
- 轮询在页面不可见（`document.visibilityState !== 'visible'`）或上一次请求未结束时跳过，防止请求堆积。
- 用户操作（连接、切换模式、选择节点）在成功后立即用返回的快照更新界面，不等下一次轮询。

## 交互约束

- 启用 TUN 前必须弹出确认框，提示先关闭 Clash Verge 的 TUN；确认后才以 `StartProxy(true)` 触发起动。
- Verge 模式下 TUN 开关禁用并说明由 Verge 管理，不能显示为 ai-cage 已启用。
- 连接运行中不允许修改模式、端口、TUN 等需要重启内核的设置。
- 危险操作（删除订阅、移除应用、退出并停止 sidecar）使用 `Modal` 二次确认，并说明实际影响范围。

## 新手教程

- 教程位于 `tutorial.tsx`，依次说明购买订阅、导入配置、关闭 Verge TUN、`AI-CAGE` 组的作用、打开沙盒窗口。
- 订阅购买链接集中定义在文件顶部的 `purchaseURL`，修改时只改这一处。
- 教程必须保留合规提醒：不提供规避检测、封号或绕过风控的内容。

## 样式

- 颜色、阴影、边框统一使用 `:root` 中的 CSS 变量；深浅色主题各定义一组。组件里写死的色值目前只有流量图 SVG 渐变（`TrafficChart`），改动时保持与 `--accent` 接近。
- 布局沿用现有 `panel`、`page-header`、`field`、`button` 等类名，新增页面优先复用，不引入 UI 组件库。
- 字体优先 `Segoe UI Variable` / `Microsoft YaHei UI`，保证 Windows 中文显示一致。

## 验证

```bash
npm --prefix frontend run build
```

`tsc` 与生产构建都必须通过。界面改动需在 Wails 桌面程序中实际打开确认，不能只凭构建成功判断。构建产物 `frontend/dist` 已被 `.gitignore` 忽略，不要提交。