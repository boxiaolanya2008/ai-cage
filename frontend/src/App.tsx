import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AppService } from '../bindings/github.com/boxiaolanya2008/ai-cage/internal/app'
import { BrandMark, EmptyState, Field, Icon, Modal, PageHeader, Toasts, Toggle } from './components'
import type { IconName } from './components'
import { KernelMode, ProxyMode, SubscriptionKind } from './types'
import { Tutorial } from './tutorial'
import type { DelayResult, LaunchApp, Proxies, Settings, Snapshot, Subscription } from './types'
import './styles.css'

type PageId = 'dashboard' | 'subscriptions' | 'nodes' | 'apps' | 'settings' | 'tutorial'
type ToastItem = { id: number; message: string; kind: 'success' | 'error' | 'info' }

const navigation: { id: PageId; label: string; icon: IconName }[] = [
  { id: 'dashboard', label: '仪表盘', icon: 'dashboard' },
  { id: 'subscriptions', label: '订阅', icon: 'subscription' },
  { id: 'nodes', label: '节点', icon: 'nodes' },
  { id: 'apps', label: '应用', icon: 'apps' },
  { id: 'settings', label: '设置', icon: 'settings' },
  { id: 'tutorial', label: '新手教程', icon: 'info' },
]

const pageCopy: Record<PageId, { title: string; description: string; eyebrow: string }> = {
  dashboard: { title: '仪表盘', description: '连接状态与实时流量，一目了然。', eyebrow: 'OVERVIEW' },
  subscriptions: { title: '订阅管理', description: '管理独立内核使用的订阅与更新。', eyebrow: 'SUBSCRIPTIONS' },
  nodes: { title: '代理节点', description: '选择代理组与节点，检查连接延迟。', eyebrow: 'PROXIES' },
  apps: { title: '普通应用', description: '以当前用户权限启动桌面应用。', eyebrow: 'APPLICATIONS' },
  settings: { title: '设置', description: '配置连接方式、独立内核与 TUN。', eyebrow: 'PREFERENCES' },
  tutorial: { title: '新手教程', description: '从订阅购买到沙盒窗口，一步步学会使用 AI Cage。', eyebrow: 'GETTING STARTED' },
}

const initialSettings: Settings = {
  kernelMode: KernelMode.KernelModeVerge,
  verge: { controllerUrl: 'http://127.0.0.1:9097', secret: '' },
  sidecar: { mihomoPath: '', mihomoSha256: '', controllerPort: 19097, mixedPort: 17897, logLevel: 'info', activeSubscription: '' },
  tun: { enabled: false, onlyCagedApps: true, stack: 'gvisor', deviceName: 'ai-cage', autoRoute: true, strictRoute: true, autoDetectInterface: true, ipv6: false, primaryDns: '223.5.5.5', secondaryDns: '119.29.29.29' },
}

const proxyModes: { value: ProxyMode; label: string; detail: string }[] = [
  { value: ProxyMode.ProxyModeRule, label: '规则', detail: '按规则分流' },
  { value: ProxyMode.ProxyModeGlobal, label: '全局', detail: '所有流量代理' },
  { value: ProxyMode.ProxyModeDirect, label: '直连', detail: '所有流量直连' },
]

function formatBytes(value: number, rate = false) {
  if (!Number.isFinite(value) || value <= 0) return rate ? '0 B/s' : '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  const amount = value / 1024 ** index
  return `${amount >= 100 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}${rate ? '/s' : ''}`
}

function formatDate(value: string) {
  if (!value) return '尚未更新'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '尚未更新'
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(date)
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error || '操作失败，请稍后重试')
}

function makeId() {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function modeLabel(mode: KernelMode | undefined) {
  return mode === KernelMode.KernelModeSidecar ? '独立内核' : 'Verge 连接'
}

function TrafficChart({ values }: { values: number[] }) {
  const samples = values.length > 1 ? values : [0, 0]
  const width = 660
  const height = 112
  const max = Math.max(...samples, 1024)
  const points = samples.map((value, index) => `${(index / (samples.length - 1)) * width},${height - (value / max) * 92}`).join(' ')
  const area = `0,${height} ${points} ${width},${height}`
  return <svg className="traffic-chart" viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-label="最近的下行流量趋势">
    <defs><linearGradient id="traffic-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#57d6b0" stopOpacity=".24"/><stop offset="1" stopColor="#57d6b0" stopOpacity="0"/></linearGradient></defs>
    <path d={`M0 28H${width} M0 60H${width} M0 92H${width}`} className="chart-grid"/>
    <polygon points={area} fill="url(#traffic-fill)"/>
    <polyline points={points} className="chart-line"/>
  </svg>
}

export default function App() {
  const [page, setPage] = useState<PageId>('dashboard')
  const [settings, setSettings] = useState<Settings>(initialSettings)
  const [draft, setDraft] = useState<Settings>(initialSettings)
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState<string | null>(null)
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const [tunConfirm, setTunConfirm] = useState(false)
  const [trafficHistory, setTrafficHistory] = useState<number[]>([])
  const [exitIP, setExitIP] = useState('')
  const toastId = useRef(0)
  const lastErrorToast = useRef(0)
  const polling = useRef(false)

  const notify = useCallback((message: string, kind: ToastItem['kind'] = 'success') => {
    const id = ++toastId.current
    setToasts(current => [...current.slice(-2), { id, message, kind }])
    window.setTimeout(() => setToasts(current => current.filter(item => item.id !== id)), kind === 'error' ? 6000 : 3800)
  }, [])

  const applySettings = useCallback((latest: Settings) => {
    setSettings(latest)
    setDraft(latest)
  }, [])

  const refreshSnapshot = useCallback(async (reportErrors = true) => {
    try {
      const next = await AppService.GetSnapshot()
      setSnapshot(next)
      setTrafficHistory(current => [...current.slice(-39), next.traffic.down || 0])
      return next
    } catch (error) {
      if (reportErrors && Date.now() - lastErrorToast.current > 6000) {
        notify(errorMessage(error), 'error')
        lastErrorToast.current = Date.now()
      }
      return null
    }
  }, [notify])

  useEffect(() => {
    let active = true
    async function initialize() {
      try {
        const loaded = await AppService.GetSettings()
        if (!active) return
        setSettings(loaded)
        setDraft(loaded)
        await refreshSnapshot()
      } catch (error) {
        if (active) notify(errorMessage(error), 'error')
      } finally {
        if (active) setLoading(false)
      }
    }
    void initialize()
    return () => { active = false }
  }, [notify, refreshSnapshot])

  useEffect(() => {
    if (!snapshot?.running) return
    const timer = window.setInterval(() => {
      if (document.visibilityState !== 'visible' || polling.current) return
      polling.current = true
      void refreshSnapshot().finally(() => { polling.current = false })
    }, 2000)
    return () => window.clearInterval(timer)
  }, [snapshot?.running, refreshSnapshot])

  const connect = useCallback(async (confirmTUN = false) => {
    setBusy('connect')
    try {
      const next = await AppService.StartProxy(confirmTUN)
      setSnapshot(next)
      setTunConfirm(false)
      notify(settings.kernelMode === KernelMode.KernelModeVerge ? '已连接 Clash Verge 控制器' : '独立内核已启动')
      await refreshSnapshot(false)
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setBusy(null)
    }
  }, [notify, refreshSnapshot, settings.kernelMode])

  const checkExitIP = async () => {
    setBusy('exit-ip')
    try {
      const value = await AppService.CheckExitIP()
      setExitIP(value)
      notify(`当前出口 IP：${value}`)
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setBusy(null)
    }
  }

  const requestConnect = () => {
    if (settings.kernelMode === KernelMode.KernelModeSidecar && settings.tun.enabled) setTunConfirm(true)
    else void connect(false)
  }

  const disconnect = async () => {
    setBusy('disconnect')
    try {
      await AppService.StopProxy()
      setSnapshot(current => current ? { ...current, running: false, coreElevated: false, message: '已断开连接' } : current)
      notify(settings.kernelMode === KernelMode.KernelModeVerge ? '已断开与 Verge 的连接，Verge 内核仍在运行' : '独立内核已停止')
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setBusy(null)
    }
  }

  const testConnection = async () => {
    setBusy('test')
    try {
      const next = await AppService.TestConnection()
      setSnapshot(current => ({ ...next, running: current?.running ?? false, coreElevated: current?.running ? next.coreElevated : false }))
      notify('控制器连接正常')
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setBusy(null)
    }
  }

  const setProxyMode = async (mode: ProxyMode) => {
    if (mode === snapshot?.proxyMode) return
    setBusy(`mode-${mode}`)
    try {
      await AppService.SetProxyMode(mode)
      setSnapshot(current => current ? { ...current, proxyMode: mode } : current)
      notify(`已切换为${proxyModes.find(item => item.value === mode)?.label}模式`)
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setBusy(null)
    }
  }

  const saveSettings = async (test = false) => {
    setBusy(test ? 'save-test' : 'save')
    try {
      await AppService.SaveSettings(draft)
      setSettings(draft)
      notify('设置已保存')
      if (test) {
        const next = await AppService.TestConnection()
        setSnapshot(current => ({ ...next, running: current?.running ?? false, coreElevated: current?.running ? next.coreElevated : false }))
        notify('设置已保存，控制器连接正常')
      }
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setBusy(null)
    }
  }

  const calculateHash = async () => {
    if (!draft.sidecar.mihomoPath.trim()) {
      notify('请先填写 mihomo 可执行文件路径', 'error')
      return
    }
    setBusy('hash')
    try {
      const hash = await AppService.CalculateMihomoSHA256(draft.sidecar.mihomoPath.trim())
      setDraft(current => ({ ...current, sidecar: { ...current.sidecar, mihomoSha256: hash } }))
      notify('SHA-256 已计算，请保存设置')
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setBusy(null)
    }
  }

  const changeMode = (kernelMode: KernelMode) => {
    setDraft(current => ({
      ...current,
      kernelMode,
      tun: kernelMode === KernelMode.KernelModeVerge
        ? { ...current.tun, enabled: false }
        : { ...current.tun, enabled: true },
    }))
  }

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><BrandMark/><div><strong>AI Cage</strong><span>代理工作台</span></div></div>
      <nav className="main-nav" aria-label="主导航">{navigation.map(item => <button key={item.id} className={`nav-item ${page === item.id ? 'active' : ''}`} onClick={() => setPage(item.id)} aria-current={page === item.id ? 'page' : undefined}><Icon name={item.icon}/><span>{item.label}</span>{page === item.id && <span className="nav-indicator"/>}</button>)}</nav>
      <div className="sidebar-bottom">
        <div className={`sidebar-status ${snapshot?.running ? 'online' : ''}`}>
          <div className="sidebar-status-head"><span className="status-dot"/><span>{snapshot?.running ? '连接正常' : '尚未连接'}</span><Icon name="mode" size={15}/>
          </div>
          <div className="sidebar-mode">{snapshot?.mode ? modeLabel(snapshot.mode) : modeLabel(settings.kernelMode)}</div>
          <button onClick={() => setPage('dashboard')}>查看状态<Icon name="chevron" size={14}/></button>
        </div>
        <div className="sidebar-foot"><span>本地运行</span><span>v0.1.0</span></div>
      </div>
    </aside>

    <main className="main-content">
      <div className="window-bar"><div className="window-title"><span className="tiny-dot"/>AI Cage<span className="window-divider"/>{pageCopy[page].title}</div><div className="window-hint">WINDOWS DESKTOP</div></div>
      <div className="content-scroll">
        {loading ? <div className="loading-screen"><span className="spinner spinner-large"/><p>正在读取本地配置</p></div> : <>
          {page === 'dashboard' && <Dashboard settings={settings} snapshot={snapshot} busy={busy} trafficHistory={trafficHistory} exitIP={exitIP} onConnect={requestConnect} onDisconnect={disconnect} onTest={testConnection} onModeChange={setProxyMode} onCheckExitIP={checkExitIP}/>}
          {page === 'subscriptions' && <Subscriptions settings={settings} busy={busy} notify={notify} onSettingsChanged={applySettings}/>}
          {page === 'nodes' && <Nodes snapshot={snapshot} busy={busy} notify={notify} onConnect={requestConnect}/>}
          {page === 'apps' && <Apps snapshot={snapshot} busy={busy} notify={notify}/>}
          {page === 'settings' && <SettingsPage settings={settings} snapshot={snapshot} draft={draft} setDraft={setDraft} busy={busy} onSave={saveSettings} onHash={calculateHash} onModeChange={changeMode}/>}
          {page === 'tutorial' && <Tutorial/>}
        </>}
        <footer className="content-footer"><span>AI Cage · 流量由本机内核处理</span><span>Controller 仅监听回环地址</span></footer>
      </div>
    </main>

    <Toasts items={toasts} dismiss={id => setToasts(current => current.filter(item => item.id !== id))}/>
    {tunConfirm && <Modal title="确认启用 TUN？" description="请先在 Clash Verge 中关闭 TUN。两个 TUN 同时接管路由可能导致断网、DNS 泄漏或规则冲突。ai-cage 不会自动关闭 Verge TUN。" confirmLabel="已关闭 Verge TUN，继续" busy={busy === 'connect'} onClose={() => setTunConfirm(false)} onConfirm={() => void connect(true)}><div className="confirm-note"><Icon name="shield" size={18}/><span>确认后，ai-cage 可能通过 UAC 请求启动提权 core。</span></div></Modal>}
  </div>
}

function Dashboard({ settings, snapshot, busy, trafficHistory, exitIP, onConnect, onDisconnect, onTest, onModeChange, onCheckExitIP }: { settings: Settings; snapshot: Snapshot | null; busy: string | null; trafficHistory: number[]; exitIP: string; onConnect: () => void; onDisconnect: () => void; onTest: () => void; onModeChange: (mode: ProxyMode) => void; onCheckExitIP: () => void }) {
  const running = snapshot?.running ?? false
  const copy = pageCopy.dashboard
  return <>
    <PageHeader eyebrow={copy.eyebrow} title={copy.title} description={copy.description} actions={<><button className="button button-secondary" onClick={onCheckExitIP} disabled={!running || busy !== null}>{busy === 'exit-ip' ? <span className="spinner"/> : <Icon name="link"/>}检测出口 IP</button><button className="button button-secondary" onClick={onTest} disabled={busy !== null}>{busy === 'test' ? <span className="spinner"/> : <Icon name="link"/>}测试连接</button></>}/>
    <section className={`connection-hero ${running ? 'is-running' : ''}`}>
      <div className="connection-identity"><div className="connection-emblem"><Icon name={running ? 'shield' : 'mode'} size={29}/><span className="emblem-orbit"/></div><div><div className="connection-title"><h2>{running ? '代理连接已就绪' : '准备连接'}</h2><span className={`status-badge ${running ? 'success' : 'neutral'}`}><span className="status-dot"/>{running ? '运行中' : '已停止'}</span></div><p>{running ? snapshot?.message || '控制器响应正常' : settings.kernelMode === KernelMode.KernelModeVerge ? '连接现有 Verge 控制器，不改变其内核状态' : '使用 ai-cage 订阅与独立内核'}</p></div></div>
      <div className="hero-actions">{running ? <button className="button button-stop" onClick={onDisconnect} disabled={busy !== null}>{busy === 'disconnect' ? <span className="spinner"/> : <Icon name="stop"/>}{settings.kernelMode === KernelMode.KernelModeVerge ? '断开连接' : '停止内核'}</button> : <button className="button button-primary button-large" onClick={onConnect} disabled={busy !== null}>{busy === 'connect' ? <span className="spinner"/> : <Icon name="play"/>}{settings.kernelMode === KernelMode.KernelModeVerge ? '连接 Verge' : '启动代理'}</button>}</div>
      <div className="hero-meta"><span><Icon name="mode" size={15}/>{modeLabel(snapshot?.mode || settings.kernelMode)}</span><span className={snapshot?.coreElevated ? 'elevated' : ''}><Icon name="shield" size={15}/>Core {snapshot?.coreElevated ? '已提权' : '未提权'}</span><span><Icon name="tun" size={15}/>TUN {snapshot?.tunEnabled ? '已启用' : '未启用'}</span>{exitIP && <span><Icon name="link" size={15}/>出口 {exitIP}</span>}</div>
    </section>

    <section className="stats-grid" aria-label="流量统计">
      <StatCard icon="arrowDown" label="实时下行" value={formatBytes(snapshot?.traffic.down || 0, true)} detail={`累计 ${formatBytes(snapshot?.traffic.downTotal || 0)}`} tone="green"/>
      <StatCard icon="arrowUp" label="实时上行" value={formatBytes(snapshot?.traffic.up || 0, true)} detail={`累计 ${formatBytes(snapshot?.traffic.upTotal || 0)}`} tone="blue"/>
      <StatCard icon="link" label="活动连接" value={String(snapshot?.connectionCount || 0)} detail={running ? '当前内核连接数' : '连接后显示'} tone="violet"/>
      <StatCard icon="shield" label="出口 IP" value={exitIP || '未检测'} detail={running ? '由 AI-CAGE 当前节点决定' : '启动后可检测'} tone="green"/>
    </section>

    <div className="dashboard-grid">
      <section className="panel traffic-panel"><div className="panel-heading"><div><h3>流量趋势</h3><p>每 2 秒采样 · 最近 80 秒</p></div><span className="live-label"><span className={running ? 'status-dot' : 'status-dot muted'}/>{running ? 'LIVE' : 'PAUSED'}</span></div><div className="chart-value"><span className="legend-dot"/>{running ? formatBytes(snapshot?.traffic.down || 0, true) : '等待连接'}</div><TrafficChart values={trafficHistory}/><div className="chart-axis"><span>80 秒前</span><span>现在</span></div></section>
      <section className="panel mode-panel"><div className="panel-heading"><div><h3>代理模式</h3><p>决定流量的走向</p></div><Icon name="mode"/></div><div className="mode-options">{proxyModes.map(item => <button key={item.value} className={`mode-option ${snapshot?.proxyMode === item.value ? 'selected' : ''}`} onClick={() => onModeChange(item.value)} disabled={!running || busy !== null}><span className="radio-dot"/><span><strong>{item.label}</strong><small>{item.detail}</small></span>{busy === `mode-${item.value}` && <span className="spinner"/>}</button>)}</div>{!running && <p className="inline-note"><Icon name="info" size={14}/>启动连接后可切换模式</p>}</section>
    </div>

    <section className="panel environment-panel"><div className="panel-heading"><div><h3>运行环境</h3><p>当前连接的内核信息</p></div><span className="subtle-badge">{settings.kernelMode === KernelMode.KernelModeVerge ? 'EXTERNAL' : 'SIDECAR'}</span></div><div className="environment-grid"><EnvironmentItem label="内核模式" value={modeLabel(snapshot?.mode || settings.kernelMode)}/><EnvironmentItem label="Core 权限" value={snapshot?.coreElevated ? '已提权' : '未提权'} highlight={snapshot?.coreElevated}/><EnvironmentItem label="内核版本" value={snapshot?.version?.version || '—'} mono/><EnvironmentItem label="控制器地址" value={snapshot?.controllerUrl || (settings.kernelMode === KernelMode.KernelModeVerge ? settings.verge.controllerUrl : `http://127.0.0.1:${settings.sidecar.controllerPort}`)} mono/></div>{settings.kernelMode === KernelMode.KernelModeVerge && <div className="info-banner"><Icon name="info"/><p><strong>关于 Verge 模式</strong>启动仅建立控制器连接，不会启动或停止 Clash Verge。其 TUN 与订阅始终由 Verge 管理。</p></div>}</section>
  </>
}

function StatCard({ icon, label, value, detail, tone }: { icon: IconName; label: string; value: string; detail: string; tone: string }) {
  return <article className={`stat-card tone-${tone}`}><div className="stat-top"><span className="stat-icon"><Icon name={icon} size={20}/></span><span>{label}</span></div><strong className="stat-value">{value}</strong><p>{detail}</p></article>
}

function EnvironmentItem({ label, value, mono = false, highlight = false }: { label: string; value: string; mono?: boolean; highlight?: boolean }) {
  return <div className="environment-item"><span>{label}</span><strong className={`${mono ? 'mono' : ''} ${highlight ? 'text-green' : ''}`}>{value}</strong></div>
}

function Subscriptions({ settings, busy, notify, onSettingsChanged }: { settings: Settings; busy: string | null; notify: (message: string, kind?: ToastItem['kind']) => void; onSettingsChanged: (settings: Settings) => void }) {
  const [items, setItems] = useState<Subscription[]>([])
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [loaded, setLoaded] = useState(false)
  const [deleting, setDeleting] = useState<Subscription | null>(null)
  const isVerge = settings.kernelMode === KernelMode.KernelModeVerge
  const copy = pageCopy.subscriptions

  const refreshItems = useCallback(async () => {
    try { setItems((await AppService.ListSubscriptions()) ?? []) } catch (error) { notify(errorMessage(error), 'error') }
  }, [notify])

  const load = useCallback(async () => {
    if (isVerge) return
    try { await refreshItems() } finally { setLoaded(true) }
  }, [isVerge, refreshItems])

  useEffect(() => { void load() }, [load])

  const add = async () => {
    if (!name.trim() || !url.trim()) { notify('请填写订阅名称和链接', 'error'); return }
    setBusyLocal('add')
    try {
      await AppService.AddSubscription(name.trim(), url.trim())
      setName(''); setUrl('')
      notify('订阅已添加并下载')
      await load()
      const latest = await AppService.GetSettings()
      onSettingsChanged(latest)
    } catch (error) { notify(errorMessage(error), 'error') } finally { setBusyLocal(null) }
  }

  const [localBusy, setLocalBusy] = useState<string | null>(null)
  function setBusyLocal(value: string | null) { setLocalBusy(value) }
  const working = busy || localBusy

  const update = async (item: Subscription) => {
    setLocalBusy(item.id)
    try { await AppService.UpdateSubscription(item.id); notify(`“${item.name}”已更新`); await load() } catch (error) { notify(errorMessage(error), 'error') } finally { setLocalBusy(null) }
  }

  const use = async (item: Subscription) => {
    setLocalBusy(item.id)
    try { await AppService.UseSubscription(item.id); onSettingsChanged(await AppService.GetSettings()); notify(`已启用“${item.name}”`) } catch (error) { notify(errorMessage(error), 'error') } finally { setLocalBusy(null) }
  }

  const importVerge = async () => {
    setLocalBusy('import')
    try {
      const item = await AppService.ImportVergeSnapshot()
      onSettingsChanged(await AppService.GetSettings())
      await refreshItems()
      setLoaded(true)
      notify(`已导入“${item.name}”；请关闭 Verge TUN 后启动 AI Cage`)
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setLocalBusy(null)
    }
  }

  const remove = async () => {
    if (!deleting) return
    setLocalBusy(deleting.id)
    try { await AppService.DeleteSubscription(deleting.id); setDeleting(null); notify('订阅已删除'); await load(); onSettingsChanged(await AppService.GetSettings()) } catch (error) { notify(errorMessage(error), 'error') } finally { setLocalBusy(null) }
  }

  return <>
    <PageHeader eyebrow={copy.eyebrow} title={copy.title} description={copy.description}/>
    {isVerge ? <section className="panel managed-panel"><div className="managed-illustration"><Icon name="subscription" size={36}/><span><Icon name="external" size={16}/></span></div><span className="eyebrow">VERGE SNAPSHOT</span><h2>一键导入 Verge 当前配置</h2><p>AI Cage 将只读导入 Clash Verge 当前启用的配置快照，并切换为独立内核模式。之后请关闭 Verge TUN，再启动 AI Cage TUN，让应用笼使用你选择的订阅节点。</p><button className="button button-primary" onClick={importVerge} disabled={working !== null}>{localBusy === 'import' ? <span className="spinner"/> : <Icon name="external" size={16}/>}导入并切换</button></section> : <>
      <section className="panel add-subscription"><div className="panel-heading"><div><h3>订阅来源</h3><p>可手动添加，或只读导入 Clash Verge 当前配置</p></div><button className="button button-secondary" onClick={importVerge} disabled={working !== null}>{localBusy === 'import' ? <span className="spinner"/> : <Icon name="external" size={16}/>}导入 Verge 当前配置</button></div><div className="subscription-form"><Field label="订阅名称" className="grow-small"><input value={name} onChange={event => setName(event.target.value)} placeholder="例如：日常订阅" maxLength={80}/></Field><Field label="订阅链接" className="grow-large"><input value={url} onChange={event => setUrl(event.target.value)} placeholder="https://example.com/subscription" type="url" autoComplete="off"/></Field><button className="button button-primary" onClick={add} disabled={working !== null}>{working === 'add' ? <span className="spinner"/> : <Icon name="plus"/>}添加订阅</button></div></section>
      <div className="section-label"><h3>我的订阅</h3><span>{items.length} 个订阅</span></div>
      {items.length === 0 && !loaded ? <div className="loading-inline"><span className="spinner"/>正在读取订阅</div> : items.length === 0 ? <EmptyState icon="subscription" title="还没有订阅" description="在上方添加订阅，启用后即可启动独立内核。"/> : <div className="subscription-list">{items.map(item => { const active = settings.sidecar.activeSubscription === item.id; return <article className={`subscription-card ${active ? 'active' : ''}`} key={item.id}><div className="subscription-icon"><Icon name="file" size={23}/></div><div className="subscription-main"><div className="subscription-title"><h3>{item.name}</h3>{item.kind === SubscriptionKind.SubscriptionKindVerge && <span className="subtle-badge">VERGE SNAPSHOT</span>}{active && <span className="status-badge success"><Icon name="check" size={12}/>已启用</span>}</div><p className="subscription-url" title={item.url}>{item.url}</p><span className="subscription-date"><Icon name="clock" size={13}/>更新于 {formatDate(item.updatedAt)}</span></div><div className="subscription-actions"><button className="icon-button" title={item.kind === SubscriptionKind.SubscriptionKindVerge ? 'Verge 快照请重新导入' : '更新订阅'} aria-label={`更新 ${item.name}`} onClick={() => update(item)} disabled={working !== null || item.kind === SubscriptionKind.SubscriptionKindVerge}>{working === item.id ? <span className="spinner"/> : <Icon name="refresh"/>}</button><button className="icon-button danger-hover" title="删除订阅" aria-label={`删除 ${item.name}`} onClick={() => setDeleting(item)} disabled={working !== null}><Icon name="trash"/></button>{!active && <button className="button button-secondary button-small" onClick={() => use(item)} disabled={working !== null}>启用</button>}</div></article> })}</div>}
    </>}
    {deleting && <Modal title="删除这个订阅？" description={`“${deleting.name}”的本地缓存将被删除。若它正在启用，独立内核将无法启动，直到你重新选择订阅。`} danger confirmLabel="删除订阅" busy={localBusy === deleting.id} onClose={() => setDeleting(null)} onConfirm={remove}/>}
  </>
}

function Nodes({ snapshot, busy, notify, onConnect }: { snapshot: Snapshot | null; busy: string | null; notify: (message: string, kind?: ToastItem['kind']) => void; onConnect: () => void }) {
  const [data, setData] = useState<Proxies>({ proxies: {} })
  const [selected, setSelected] = useState('')
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)
  const [delays, setDelays] = useState<Record<string, number | 'testing' | 'error'>>({})
  const running = snapshot?.running ?? false
  const copy = pageCopy.nodes

  const groups = useMemo(() => {
    const list = Object.values(data.proxies).filter(item => item.all?.length > 0).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
    const cageIndex = list.findIndex(item => item.name === 'AI-CAGE')
    if (cageIndex > 0) {
      const [cage] = list.splice(cageIndex, 1)
      list.unshift(cage)
    }
    return list
  }, [data])
  const current = data.proxies[selected]

  const load = useCallback(async () => {
    if (!running) return
    setLoading(true)
    try {
      const result = await AppService.GetProxies()
      const proxies: Proxies['proxies'] = {}
      for (const [name, proxy] of Object.entries(result.proxies ?? {})) {
        if (proxy) proxies[name] = { ...proxy, all: proxy.all ?? [] }
      }
      setData({ proxies })
      setSelected(value => proxies[value]?.all.length ? value : Object.values(proxies).find(item => item.all.length > 0)?.name || '')
    } catch (error) { notify(errorMessage(error), 'error') } finally { setLoading(false) }
  }, [notify, running])

  useEffect(() => { void load() }, [load])

  const select = async (name: string) => {
    if (!current || name === current.now) return
    setBusyProxy(name)
    try { await AppService.SelectProxy(current.name, name); setData(previous => ({ proxies: { ...previous.proxies, [current.name]: { ...current, now: name } } })); notify(`已切换到“${name}”`) } catch (error) { notify(errorMessage(error), 'error') } finally { setBusyProxy(null) }
  }

  const [selecting, setSelecting] = useState<string | null>(null)
  function setBusyProxy(value: string | null) { setSelecting(value) }

  const testDelay = async (name: string) => {
    if (!current) return
    setDelays(previous => ({ ...previous, [name]: 'testing' }))
    try {
      const result: DelayResult = await AppService.TestProxyDelay(current.name, name)
      setDelays(previous => ({ ...previous, [name]: result.delay }))
    } catch (error) {
      setDelays(previous => ({ ...previous, [name]: 'error' }))
      notify(`${name}：${errorMessage(error)}`, 'error')
    }
  }

  const visibleNodes = current?.all.filter(name => name.toLowerCase().includes(query.toLowerCase())) || []

  return <>
    <PageHeader eyebrow={copy.eyebrow} title={copy.title} description={copy.description} actions={<button className="button button-secondary" onClick={load} disabled={!running || loading}>{loading ? <span className="spinner"/> : <Icon name="refresh"/>}刷新节点</button>}/>
    {!running ? <EmptyState icon="nodes" title="连接内核后可选择节点" description="先在仪表盘启动代理连接，再查看代理组与节点延迟。" action={<button className="button button-primary" onClick={onConnect} disabled={busy !== null}><Icon name="play"/>前往仪表盘</button>}/> : <div className="nodes-layout">
      <aside className="panel group-panel"><div className="panel-heading"><div><h3>代理组</h3><p>{groups.length} 个可选组</p></div></div><div className="group-list">{groups.map(group => <button className={`group-item ${selected === group.name ? 'active' : ''}`} key={group.name} onClick={() => { setSelected(group.name); setQuery(''); setDelays({}) }}><span className="group-icon"><Icon name="nodes" size={17}/></span><span><strong>{group.name}</strong><small>{group.type} · {group.all.length} 个节点</small></span><Icon name="chevron" size={14}/></button>)}{!loading && groups.length === 0 && <p className="panel-empty">当前订阅没有可选代理组</p>}</div></aside>
      <section className="panel node-panel"><div className="node-header"><div><span className="eyebrow">PROXY GROUP</span><h2>{current?.name || '选择代理组'}</h2><p>{current ? `当前：${current.now}` : '从左侧选择代理组'}</p></div>{current && <span className={`status-badge ${current.alive ? 'success' : 'warning'}`}>{current.alive ? '组可用' : '组不可用'}</span>}</div><div className="node-toolbar"><label className="search-box"><Icon name="search" size={17}/><input value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索节点名称" disabled={!current}/></label><span>{visibleNodes.length} 个节点</span></div><div className="node-list">{visibleNodes.map(name => { const active = name === current?.now; const delay = delays[name]; return <div key={name} className={`node-row ${active ? 'active' : ''}`}><span className="node-radio">{active && <span/>}</span><div className="node-name"><strong>{name}</strong>{active && <small>当前使用</small>}</div><button className={`delay-button ${delay === 'testing' ? 'testing' : ''} ${typeof delay === 'number' ? delay < 300 ? 'fast' : delay < 800 ? 'medium' : 'slow' : ''}`} onClick={() => testDelay(name)} disabled={selecting !== null} title="测试节点延迟">{delay === 'testing' ? <span className="spinner"/> : typeof delay === 'number' ? `${delay} ms` : delay === 'error' ? '超时' : '测延迟'}</button><button className="button button-secondary button-small" onClick={() => select(name)} disabled={active || selecting !== null}>{selecting === name ? <span className="spinner"/> : '使用'}</button></div> })}{!loading && current && visibleNodes.length === 0 && <p className="panel-empty">没有匹配的节点</p>}{loading && <div className="loading-inline"><span className="spinner"/>正在读取节点</div>}</div></section>
    </div>}
  </>
}

function Apps({ snapshot, busy, notify }: { snapshot: Snapshot | null; busy: string | null; notify: (message: string, kind?: ToastItem['kind']) => void }) {
  const running = snapshot?.running ?? false
  const [items, setItems] = useState<LaunchApp[]>([])
  const [editing, setEditing] = useState<LaunchApp | null>(null)
  const [deleting, setDeleting] = useState<LaunchApp | null>(null)
  const [form, setForm] = useState({ name: '', path: '', args: '' })
  const [loaded, setLoaded] = useState(false)
  const copy = pageCopy.apps

  const load = useCallback(async () => {
    try { setItems(((await AppService.ListApps()) ?? []).map(item => ({ ...item, args: item.args ?? [] }))) } catch (error) { notify(errorMessage(error), 'error') } finally { setLoaded(true) }
  }, [notify])

  useEffect(() => { void load() }, [load])

  const openAdd = () => { setEditing({ id: '', name: '', path: '', args: [] }); setForm({ name: '', path: '', args: '' }) }
  const openEdit = (item: LaunchApp) => { setEditing(item); setForm({ name: item.name, path: item.path, args: item.args.join(' ') }) }
  const save = async () => {
    if (!editing || !form.name.trim() || !form.path.trim()) { notify('请填写应用名称和可执行文件路径', 'error'); return }
    setAppBusy('save')
    try {
      const app: LaunchApp = { id: editing.id || makeId(), name: form.name.trim(), path: form.path.trim(), args: form.args.trim() ? form.args.trim().split(/\s+/) : [] }
      await AppService.SaveApp(app)
      setEditing(null)
      notify(editing.id ? '应用信息已保存' : '应用已添加')
      await load()
    } catch (error) { notify(errorMessage(error), 'error') } finally { setAppBusy(null) }
  }
  const launch = async (item: LaunchApp) => {
    if (!running) { notify('请先启动 AI Cage 代理，再打开应用笼窗口', 'error'); return }
    setAppBusy(item.id)
    try {
      await AppService.OpenSandboxWindow(item.id)
      notify(`正在为“${item.name}”打开沙盒窗口，流量将通过 AI-CAGE 当前节点`)
    } catch (error) {
      notify(errorMessage(error), 'error')
    } finally {
      setAppBusy(null)
    }
  }
  const remove = async () => {
    if (!deleting) return
    setAppBusy(deleting.id)
    try { await AppService.DeleteApp(deleting.id); setDeleting(null); notify('应用已移除'); await load() } catch (error) { notify(errorMessage(error), 'error') } finally { setAppBusy(null) }
  }
  const [appBusy, setAppBusy] = useState<string | null>(null)
  const working = busy || appBusy

  return <>
    <PageHeader eyebrow={copy.eyebrow} title={copy.title} description={copy.description} actions={<button className="button button-primary" onClick={openAdd} disabled={working !== null}><Icon name="plus"/>添加应用</button>}/>
    <div className="info-banner app-info"><Icon name="shield"/><p><strong>AI Cage 沙盒窗口</strong>应用会在独立的 AI Cage 宿主窗口中打开，窗口内只显示这个应用。它不是安全沙盒，也不会解除应用自身的系统限制；启用“仅应用笼流量走代理”后，窗口内应用会通过 AI-CAGE 当前节点。</p></div>
    {items.length === 0 && !loaded ? <div className="loading-inline"><span className="spinner"/>正在读取应用</div> : items.length === 0 ? <EmptyState icon="apps" title="还没有常用应用" description="添加应用的可执行文件路径，即可在这里快速启动。" action={<button className="button button-secondary" onClick={openAdd}><Icon name="plus"/>添加第一个应用</button>}/> : <div className="apps-grid">{items.map(item => <article className="app-card" key={item.id}><div className="app-card-head"><div className="app-icon"><Icon name="apps" size={24}/></div><div className="app-actions"><button className="icon-button" title="编辑应用" aria-label={`编辑 ${item.name}`} onClick={() => openEdit(item)} disabled={working !== null}><Icon name="settings" size={16}/></button><button className="icon-button danger-hover" title="移除应用" aria-label={`移除 ${item.name}`} onClick={() => setDeleting(item)} disabled={working !== null}><Icon name="trash" size={16}/></button></div></div><h3>{item.name}</h3><p className="app-path" title={item.path}>{item.path}</p>{item.args.length > 0 && <p className="app-args">{item.args.join(' ')}</p>}<button className="button button-secondary launch-button" onClick={() => launch(item)} disabled={working !== null || !running} title={running ? '在独立沙盒窗口中打开' : '请先启动 AI Cage 代理'}>{appBusy === item.id ? <span className="spinner"/> : <Icon name="play" size={16}/>}打开沙盒窗口</button></article>)}</div>}
    {editing && <Modal title={editing.id ? '编辑应用' : '添加普通应用'} description="填写本机可执行文件的完整路径，可使用空格分隔启动参数。" confirmLabel="保存应用" busy={appBusy === 'save'} onClose={() => setEditing(null)} onConfirm={save}><div className="modal-form"><Field label="显示名称"><input autoFocus value={form.name} onChange={event => setForm({ ...form, name: event.target.value })} placeholder="例如：浏览器" maxLength={80}/></Field><Field label="可执行文件路径" hint="请使用 .exe 的完整路径"><input value={form.path} onChange={event => setForm({ ...form, path: event.target.value })} placeholder="C:\Program Files\App\app.exe"/></Field><Field label="启动参数（可选）"><input value={form.args} onChange={event => setForm({ ...form, args: event.target.value })} placeholder="--new-window"/></Field></div></Modal>}
    {deleting && <Modal title="移除这个应用？" description={`仅从 AI Cage 的启动列表移除“${deleting.name}”，不会删除应用文件。`} danger confirmLabel="移除应用" busy={appBusy === deleting.id} onClose={() => setDeleting(null)} onConfirm={remove}/>}
  </>
}

function SettingsPage({ settings, snapshot, draft, setDraft, busy, onSave, onHash, onModeChange }: { settings: Settings; snapshot: Snapshot | null; draft: Settings; setDraft: (settings: Settings) => void; busy: string | null; onSave: (test?: boolean) => void; onHash: () => void; onModeChange: (mode: KernelMode) => void }) {
  const running = snapshot?.running ?? false
  const isVerge = draft.kernelMode === KernelMode.KernelModeVerge
  const copy = pageCopy.settings
  const patchVerge = (key: keyof Settings['verge'], value: string) => setDraft({ ...draft, verge: { ...draft.verge, [key]: value } })
  const patchSidecar = <K extends keyof Settings['sidecar']>(key: K, value: Settings['sidecar'][K]) => setDraft({ ...draft, sidecar: { ...draft.sidecar, [key]: value } })
  const patchTUN = <K extends keyof Settings['tun']>(key: K, value: Settings['tun'][K]) => setDraft({ ...draft, tun: { ...draft.tun, [key]: value } })

  return <>
    <PageHeader eyebrow={copy.eyebrow} title={copy.title} description={copy.description} actions={<button className="button button-primary" onClick={() => onSave(false)} disabled={busy !== null || running}>{busy === 'save' ? <span className="spinner"/> : <Icon name="save"/>}保存设置</button>}/>
    {running && <div className="info-banner"><Icon name="info"/><p><strong>当前连接正在运行</strong>请先在仪表盘断开连接，再修改设置。现有配置会在停止连接后保留。</p></div>}

    <section className="panel settings-section"><div className="settings-section-head"><span className="section-icon"><Icon name="mode"/></span><div><h2>连接方式</h2><p>两种模式互斥，按你的使用环境选择。</p></div></div><div className="kernel-options"><button className={`kernel-option ${isVerge ? 'selected' : ''}`} onClick={() => onModeChange(KernelMode.KernelModeVerge)} disabled={running || busy !== null}><span className="kernel-radio"/><div className="kernel-logo verge-logo"><Icon name="external" size={23}/></div><div><strong>Verge 连接</strong><p>连接现有 Clash Verge 控制器，不管理其进程与订阅。</p></div>{isVerge && <Icon name="check" size={18}/>}</button><button className={`kernel-option ${!isVerge ? 'selected' : ''}`} onClick={() => onModeChange(KernelMode.KernelModeSidecar)} disabled={running || busy !== null}><span className="kernel-radio"/><div className="kernel-logo sidecar-logo"><Icon name="shield" size={23}/></div><div><strong>独立内核</strong><p>使用 AI Cage 的订阅、端口与 mihomo sidecar。</p></div>{!isVerge && <Icon name="check" size={18}/>}</button></div></section>

    <section className="panel settings-section"><div className="settings-section-head"><span className="section-icon"><Icon name="link"/></span><div><h2>{isVerge ? 'Verge 控制器' : '独立内核'}</h2><p>{isVerge ? 'External Controller 仅允许回环地址。' : '配置本机 mihomo 与监听端口。'}</p></div></div>{isVerge ? <><div className="form-grid"><Field label="Controller URL" hint="Clash Verge 默认通常为 http://127.0.0.1:9097"><input value={draft.verge.controllerUrl} onChange={event => patchVerge('controllerUrl', event.target.value)} placeholder="http://127.0.0.1:9097" disabled={running}/></Field><Field label="Secret" hint="需在 Verge 开启外部控制；不要使用公开占位值 set-your-secret"><input type="password" value={draft.verge.secret} onChange={event => patchVerge('secret', event.target.value)} placeholder="Verge 控制器密钥" autoComplete="off" disabled={running}/></Field></div><div className="settings-footer"><span><Icon name="info" size={14}/>测试会读取内核版本、配置与连接信息</span><button className="button button-secondary" onClick={() => onSave(true)} disabled={busy !== null || running}>{busy === 'save-test' ? <span className="spinner"/> : <Icon name="link"/>}保存并测试</button></div></> : <><div className="form-grid"><Field label="mihomo 路径" className="span-2" hint="ai-cage 只启动这个本机文件，不会自动下载内核"><div className="input-action"><Icon name="folder" size={17}/><input value={draft.sidecar.mihomoPath} onChange={event => patchSidecar('mihomoPath', event.target.value)} placeholder="D:\mihomo\mihomo.exe" disabled={running}/><button className="button button-secondary button-small" onClick={onHash} disabled={running || busy !== null}>{busy === 'hash' ? <span className="spinner"/> : <Icon name="hash" size={15}/>}计算</button></div></Field><Field label="mihomo SHA-256" className="span-2" hint="用于启动前校验内核文件，修改内核文件后需重新计算"><div className="input-action"><Icon name="hash" size={17}/><input className="mono hash-input" value={draft.sidecar.mihomoSha256} onChange={event => patchSidecar('mihomoSha256', event.target.value.trim().toLowerCase())} placeholder="64 位十六进制哈希" maxLength={64} spellCheck={false} disabled={running}/></div></Field><Field label="Controller 端口"><input type="number" min={1} max={65535} value={draft.sidecar.controllerPort} onChange={event => patchSidecar('controllerPort', Number(event.target.value))} disabled={running}/></Field><Field label="混合代理端口"><input type="number" min={1} max={65535} value={draft.sidecar.mixedPort} onChange={event => patchSidecar('mixedPort', Number(event.target.value))} disabled={running}/></Field><Field label="日志级别" className="span-2"><select value={draft.sidecar.logLevel} onChange={event => patchSidecar('logLevel', event.target.value)} disabled={running}>{['silent', 'error', 'warning', 'info', 'debug'].map(level => <option value={level} key={level}>{level}</option>)}</select></Field></div><div className="settings-footer"><span><Icon name="info" size={14}/>端口必须不同，且 Controller 只监听回环地址</span><button className="button button-secondary" onClick={() => onSave(true)} disabled={busy !== null || running}>{busy === 'save-test' ? <span className="spinner"/> : <Icon name="link"/>}保存并测试</button></div></>}</section>

    <section className={`panel settings-section tun-settings ${isVerge ? 'is-disabled-section' : ''}`}><div className="settings-section-head"><span className="section-icon"><Icon name="tun"/></span><div><h2>TUN 与 DNS</h2><p>{isVerge ? 'Verge 模式下由 Clash Verge 管理 TUN。' : 'TUN 接管网络流量，需要 Windows 网络权限。'}</p></div><Toggle label="启用 ai-cage TUN" checked={draft.tun.enabled} disabled={running || isVerge} onChange={value => patchTUN('enabled', value)}/></div>{isVerge ? <div className="info-banner compact"><Icon name="info"/><p><strong>TUN 由 Verge 管理</strong>为避免双重接管路由，Verge 模式下禁用 ai-cage TUN。请直接在 Clash Verge 中开启或关闭 TUN。</p></div> : <><div className="form-grid"><Field label="TUN 协议栈"><select value={draft.tun.stack} onChange={event => patchTUN('stack', event.target.value)} disabled={running || !draft.tun.enabled}>{['gvisor', 'system', 'mixed'].map(stack => <option key={stack} value={stack}>{stack}</option>)}</select></Field><Field label="设备名称"><input value={draft.tun.deviceName} onChange={event => patchTUN('deviceName', event.target.value)} placeholder="ai-cage" disabled={running || !draft.tun.enabled}/></Field><Field label="主 DNS"><input value={draft.tun.primaryDns} onChange={event => patchTUN('primaryDns', event.target.value.trim())} placeholder="223.5.5.5" disabled={running || !draft.tun.enabled}/></Field><Field label="备用 DNS"><input value={draft.tun.secondaryDns} onChange={event => patchTUN('secondaryDns', event.target.value.trim())} placeholder="119.29.29.29" disabled={running || !draft.tun.enabled}/></Field></div><div className="toggle-list"><SettingToggle label="自动路由" description="自动配置系统路由" checked={draft.tun.autoRoute} disabled={running || !draft.tun.enabled} onChange={value => patchTUN('autoRoute', value)}/><SettingToggle label="仅应用笼流量走代理" description="只让 AI Cage 启动的应用使用 AI-CAGE 节点，其他程序直连" checked={draft.tun.onlyCagedApps} disabled={running || !draft.tun.enabled} onChange={value => patchTUN('onlyCagedApps', value)}/><SettingToggle label="严格路由" description="按严格规则保护流量路径" checked={draft.tun.strictRoute} disabled={running || !draft.tun.enabled} onChange={value => patchTUN('strictRoute', value)}/><SettingToggle label="自动检测出口" description="跟随当前物理网卡" checked={draft.tun.autoDetectInterface} disabled={running || !draft.tun.enabled} onChange={value => patchTUN('autoDetectInterface', value)}/><SettingToggle label="IPv6" description="允许 TUN 处理 IPv6 流量" checked={draft.tun.ipv6} disabled={running || !draft.tun.enabled} onChange={value => patchTUN('ipv6', value)}/></div><div className="info-banner compact warning-banner"><Icon name="warning"/><p><strong>启动前必须手动确认</strong>若使用 sidecar TUN，请先关闭 Clash Verge 的 TUN。之后每次启动都会要求确认；启用 TUN 时 core 将通过 UAC 提权。</p></div></>}</section>
    <div className="settings-save-bar"><div><strong>{settings.kernelMode === draft.kernelMode ? '配置保存在本机' : '模式尚未保存'}</strong><span>{running ? '停止连接后可修改' : '所有字段将在保存后生效'}</span></div><button className="button button-primary" onClick={() => onSave(false)} disabled={busy !== null || running}>{busy === 'save' ? <span className="spinner"/> : <Icon name="save"/>}保存设置</button></div>
  </>
}

function SettingToggle({ label, description, checked, disabled, onChange }: { label: string; description: string; checked: boolean; disabled: boolean; onChange: (value: boolean) => void }) {
  return <div className="setting-toggle"><div><strong>{label}</strong><span>{description}</span></div><Toggle label={label} checked={checked} disabled={disabled} onChange={onChange}/></div>
}
