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
}

export interface Settings {
  site_title: string
  show_price: boolean
  default_report_interval: number
  install_script_base: string
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
}
