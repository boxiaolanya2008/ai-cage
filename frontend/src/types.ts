import type {
  DelayResult as GeneratedDelayResult,
  LaunchApp as GeneratedLaunchApp,
  Proxy as GeneratedProxy,
  Settings as GeneratedSettings,
  Snapshot as GeneratedSnapshot,
  Subscription as GeneratedSubscription,
} from '../bindings/github.com/boxiaolanya2008/ai-cage/internal/model/models'

export { KernelMode, ProxyMode, SubscriptionKind } from '../bindings/github.com/boxiaolanya2008/ai-cage/internal/model/models'
export type Settings = GeneratedSettings
export type Snapshot = GeneratedSnapshot
export type Subscription = GeneratedSubscription
export type DelayResult = GeneratedDelayResult
export type LaunchApp = Omit<GeneratedLaunchApp, 'args'> & { args: string[] }
export type Proxy = Omit<GeneratedProxy, 'all'> & { all: string[] }

export interface Proxies {
  proxies: Record<string, Proxy>
}
