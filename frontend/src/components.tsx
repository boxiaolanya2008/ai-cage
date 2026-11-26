import type { ReactNode } from 'react'

export type IconName =
  | 'dashboard' | 'subscription' | 'nodes' | 'apps' | 'settings'
  | 'play' | 'stop' | 'arrowUp' | 'arrowDown' | 'link' | 'shield'
  | 'chevron' | 'refresh' | 'plus' | 'trash' | 'check' | 'close'
  | 'search' | 'file' | 'folder' | 'hash' | 'clock' | 'info'
  | 'external' | 'mode' | 'tun' | 'save' | 'spark' | 'warning'

const paths: Record<IconName, ReactNode> = {
  dashboard: <><rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="5" rx="1.5"/><rect x="14" y="12" width="7" height="9" rx="1.5"/><rect x="3" y="16" width="7" height="5" rx="1.5"/></>,
  subscription: <><path d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v16H6.5A2.5 2.5 0 0 0 4 21.5z"/><path d="M4 5.5v16M8 7h8M8 11h6"/></>,
  nodes: <><circle cx="12" cy="5" r="2.5"/><circle cx="5" cy="18" r="2.5"/><circle cx="19" cy="18" r="2.5"/><path d="M12 7.5v4M5 15.5v-2h14v2"/></>,
  apps: <><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/></>,
  settings: <><path d="m9.5 3-.6 2.2-2 .9-2-.6-2.4 4.1 1.7 1.5v2l-1.7 1.5 2.4 4.1 2-.6 2 .9.6 2.2h5l.6-2.2 2-.9 2 .6 2.4-4.1-1.7-1.5v-2l1.7-1.5-2.4-4.1-2 .6-2-.9-.6-2.2z"/><circle cx="12" cy="12" r="3"/></>,
  play: <path d="m8 5 11 7-11 7z"/>,
  stop: <rect x="6" y="6" width="12" height="12" rx="1.5"/>,
  arrowUp: <><path d="M12 20V4m-6 6 6-6 6 6"/></>,
  arrowDown: <><path d="M12 4v16m-6-6 6 6 6-6"/></>,
  link: <><path d="M10 13a5 5 0 0 0 7.1.1l2-2a5 5 0 0 0-7.1-7.1l-1.1 1.1"/><path d="M14 11a5 5 0 0 0-7.1-.1l-2 2A5 5 0 0 0 12 20l1.1-1.1"/></>,
  shield: <><path d="m12 3 8 3v6c0 5-8 9-8 9s-8-4-8-9V6z"/><path d="m8.5 12 2.2 2.2 4.8-5"/></>,
  chevron: <path d="m9 5 7 7-7 7"/>,
  refresh: <><path d="M20 7v5h-5M4 17v-5h5"/><path d="M6.1 6.1A8 8 0 0 1 20 12M4 12a8 8 0 0 0 13.9 5.9"/></>,
  plus: <path d="M12 5v14M5 12h14"/>,
  trash: <><path d="M4 7h16M9 7V4h6v3M6 7l1 14h10l1-14M10 11v6M14 11v6"/></>,
  check: <path d="m5 12 4.5 4.5L19 7"/>,
  close: <path d="m6 6 12 12M6 18 18 6"/>,
  search: <><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/></>,
  file: <><path d="M6 3h8l4 4v14H6z"/><path d="M14 3v5h5M9 13h6M9 17h5"/></>,
  folder: <path d="M3 7V5h6l2 3h10v12H3z"/>,
  hash: <><path d="M10 3 8 21M16 3l-2 18M4 9h16M3 15h16"/></>,
  clock: <><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></>,
  info: <><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/></>,
  external: <><path d="M14 4h6v6M20 4l-9 9"/><path d="M20 14v6H4V4h6"/></>,
  mode: <><path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="2" fill="currentColor"/><circle cx="15" cy="17" r="2" fill="currentColor"/></>,
  tun: <><rect x="4" y="4" width="16" height="16" rx="3"/><path d="M8 8v8M12 8v8M16 8v8"/></>,
  save: <><path d="M4 3h13l3 3v15H4z"/><path d="M8 3v6h8V3M8 21v-7h8v7"/></>,
  spark: <><path d="m12 3 2.2 6.8L21 12l-6.8 2.2L12 21l-2.2-6.8L3 12l6.8-2.2z"/></>,
  warning: <><path d="m12 3 10 18H2z"/><path d="M12 9v5M12 17h.01"/></>,
}

export function Icon({ name, size = 18, className = '' }: { name: IconName; size?: number; className?: string }) {
  return <svg className={`icon ${className}`} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>
}

export function BrandMark() {
  return <svg className="brand-mark" viewBox="0 0 40 40" fill="none" aria-hidden="true">
    <defs><linearGradient id="brand-gradient" x1="12" y1="11" x2="28" y2="29"><stop stopColor="#93f2d3"/><stop offset=".5" stopColor="#57d6b0"/><stop offset="1" stopColor="#4aa8e0"/></linearGradient></defs>
    <path d="M18.3 12.2H13v15.6h5.3M21.7 12.2H27v15.6h-5.3" stroke="url(#brand-gradient)" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round"/>
    <circle cx="20" cy="20" r="3.8" fill="url(#brand-gradient)"/>
  </svg>
}

export function Toggle({ checked, onChange, disabled = false, label }: { checked: boolean; onChange: (checked: boolean) => void; disabled?: boolean; label: string }) {
  return <button type="button" role="switch" aria-checked={checked} aria-label={label} disabled={disabled} className={`toggle ${checked ? 'is-on' : ''}`} onClick={() => onChange(!checked)}><span /></button>
}

export function Field({ label, hint, children, className = '' }: { label: string; hint?: string; children: ReactNode; className?: string }) {
  return <label className={`field ${className}`}><span className="field-label">{label}</span>{children}{hint && <span className="field-hint">{hint}</span>}</label>
}

export function PageHeader({ eyebrow, title, description, actions }: { eyebrow: string; title: string; description: string; actions?: ReactNode }) {
  return <header className="page-header"><div><span className="eyebrow">{eyebrow}</span><h1>{title}</h1><p>{description}</p></div>{actions && <div className="page-actions">{actions}</div>}</header>
}

export function EmptyState({ icon, title, description, action }: { icon: IconName; title: string; description: string; action?: ReactNode }) {
  return <div className="empty-state"><div className="empty-icon"><Icon name={icon} size={25}/></div><h3>{title}</h3><p>{description}</p>{action}</div>
}

export interface ToastItem { id: number; message: string; kind: 'success' | 'error' | 'info' }

export function Toasts({ items, dismiss }: { items: ToastItem[]; dismiss: (id: number) => void }) {
  return <div className="toast-region" aria-live="polite" aria-atomic="false">{items.map(item => <div key={item.id} className={`toast toast-${item.kind}`} role={item.kind === 'error' ? 'alert' : 'status'}><span className="toast-symbol"><Icon name={item.kind === 'error' ? 'warning' : item.kind === 'success' ? 'check' : 'info'} size={17}/></span><span>{item.message}</span><button type="button" aria-label="关闭通知" onClick={() => dismiss(item.id)}><Icon name="close" size={15}/></button></div>)}</div>
}

export function Modal({ title, description, children, onClose, onConfirm, confirmLabel, danger = false, busy = false }: { title: string; description: string; children?: ReactNode; onClose: () => void; onConfirm: () => void; confirmLabel: string; danger?: boolean; busy?: boolean }) {
  return <div className="modal-backdrop" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget && !busy) onClose() }}><section className="modal" role="dialog" aria-modal="true" aria-labelledby="modal-title"><div className={`modal-icon ${danger ? 'danger' : ''}`}><Icon name={danger ? 'trash' : 'tun'} size={23}/></div><h2 id="modal-title">{title}</h2><p>{description}</p>{children && <div className="modal-body">{children}</div>}<div className="modal-actions"><button className="button button-secondary" onClick={onClose} disabled={busy}>取消</button><button className={`button ${danger ? 'button-danger' : 'button-primary'}`} onClick={onConfirm} disabled={busy}>{busy ? <><span className="spinner"/>处理中</> : confirmLabel}</button></div></section></div>
}
