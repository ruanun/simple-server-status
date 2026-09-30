/** 与后端 JSON 契约一一对应的类型（字段均为 snake_case） */

export interface Hello {
  os: string
  platform: string
  platform_version: string
  kernel: string
  arch: string
  virtualization: string
  cpu_model: string
  cpu_cores: number
  mem_total: number
  swap_total: number
  disk_total: number
  agent_version: string
  country: string
  ipv4?: string
  ipv6?: string
}

export interface Disk {
  mount: string
  fstype: string
  total: number
  used: number
}

export interface Metrics {
  ts: number
  cpu: number
  load1: number
  load5: number
  load15: number
  mem_used: number
  swap_used: number
  disk_used: number
  disk_total: number
  disks: Disk[]
  net_in_speed: number
  net_out_speed: number
  net_in_total: number
  net_out_total: number
  procs: number
  tcp: number
  udp: number
  uptime: number
}

export type TrafficMode = 'sum' | 'in' | 'out'

export interface Traffic {
  in: number
  out: number
  used: number
  limit: number | null
  mode: TrafficMode
  reset_day: number
  period: string
}

export type BillingCycle = '' | 'monthly' | 'quarterly' | 'yearly' | 'once'

export interface ServerView {
  id: string
  name: string
  group: string
  country: string
  sort: number
  hidden: boolean
  online: boolean
  last_seen: number
  static: Hello | null
  metrics: Metrics | null
  traffic: Traffic
  expire_at: number | null
  price?: number | null
  currency?: string
  billing_cycle?: BillingCycle
  uptime_24h: number | null
  ipv4: boolean
  ipv6: boolean
}

export interface Point {
  ts: number
  cpu: number
  mem: number
  disk: number
  net_in: number
  net_out: number
  load1: number
  tcp: number
}

export type Range = 'realtime' | '1h' | '6h' | '24h' | '7d'
export const RANGES: Range[] = ['realtime', '1h', '6h', '24h', '7d']

export interface Site {
  site_title: string
  show_price: boolean
  announcement: string
}

export interface NotifySettings {
  webhook_url: string
  telegram_token: string
  telegram_chat_id: string
  lang: 'zh-CN' | 'en-US'
  offline_enabled: boolean
  offline_minutes: number
  load_enabled: boolean
  load_cpu: number
  load_mem: number
  load_disk: number
  load_minutes: number
  expire_enabled: boolean
  expire_days: number
  traffic_enabled: boolean
  traffic_percent: number
}

export interface NotifyTestResult {
  webhook: string | null
  telegram: string | null
}

export interface Settings {
  site_title: string
  show_price: boolean
  default_report_interval: number
  install_script_base: string
  announcement: string
  notify: NotifySettings
}

export interface ServerInput {
  name: string
  group: string
  country: string
  hidden: boolean
  price: number | null
  currency: string
  billing_cycle: BillingCycle
  expire_at: number | null
  traffic_limit: number | null
  traffic_mode: TrafficMode
  traffic_reset_day: number
  report_interval: number
  nic_include: string[]
  nic_exclude: string[]
  mount_exclude: string[]
  note: string
  notify_muted: boolean
}

export interface DailyTraffic {
  day: string
  in: number
  out: number
}

export interface ServerStats {
  uptime_24h: number | null
  uptime_7d: number | null
  period_start: string
  period_end: string
  daily: DailyTraffic[]
}

export interface AdminServer extends ServerInput {
  id: string
  secret: string
  sort: number
  static_info: Hello | null
  last_ip: string
  last_seen: number
  created_at: number
  updated_at: number
  online: boolean
  agent_version: string
  outdated: boolean
  uptime_24h: number | null
}

export interface CostGroup {
  currency: string
  amount: number
  servers: number
}

export interface ExpiringServer {
  id: string
  name: string
  expire_at: number
  days: number
}

export interface Overview {
  monthly_cost: CostGroup[]
  expiring: ExpiringServer[]
}

export interface PublicOutage {
  start_at: number
  end_at: number | null
  duration: number
}

export interface AdminOutage {
  id: number
  server_id: string
  server_name: string
  start_at: number
  end_at: number | null
  duration: number
}

export type NotifyLogStatus = 'pending' | 'sent' | 'failed'

export interface NotifyLogItem {
  id: number
  server_id: string
  server_name: string
  kind: string
  channel: string
  title: string
  message: string
  status: NotifyLogStatus
  error: string
  created_at: number
  done_at: number | null
}

export interface Paged<T> {
  items: T[]
  total: number
}
