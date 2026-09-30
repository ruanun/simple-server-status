import type { AdminServer, Hello, Metrics, ServerView } from '@/lib/types'

export function makeHello(overrides: Partial<Hello> = {}): Hello {
  return {
    os: 'linux',
    platform: 'debian',
    platform_version: '12',
    kernel: '6.1.0',
    arch: 'x86_64',
    virtualization: 'kvm',
    cpu_model: 'AMD EPYC 7B13',
    cpu_cores: 4,
    mem_total: 2 * 1024 ** 3,
    swap_total: 1024 ** 3,
    disk_total: 40 * 1024 ** 3,
    agent_version: '2.0.0',
    country: 'HK',
    ...overrides,
  }
}

export function makeMetrics(overrides: Partial<Metrics> = {}): Metrics {
  return {
    ts: 1_700_000_000,
    cpu: 12,
    load1: 0.5,
    load5: 0.4,
    load15: 0.3,
    mem_used: 1024 ** 3,
    swap_used: 0,
    disk_used: 10 * 1024 ** 3,
    disk_total: 40 * 1024 ** 3,
    disks: [{ mount: '/', fstype: 'ext4', total: 40 * 1024 ** 3, used: 10 * 1024 ** 3 }],
    net_in_speed: 1024,
    net_out_speed: 2048,
    net_in_total: 10 * 1024 ** 3,
    net_out_total: 5 * 1024 ** 3,
    procs: 120,
    tcp: 30,
    udp: 4,
    uptime: 3 * 86400 + 2 * 3600,
    ...overrides,
  }
}

export function makeServer(overrides: Partial<ServerView> = {}): ServerView {
  return {
    id: 'srv1',
    name: 'hk-01',
    group: 'HK',
    country: 'HK',
    sort: 0,
    hidden: false,
    online: true,
    last_seen: 1_700_000_000,
    static: makeHello(),
    metrics: makeMetrics(),
    traffic: { in: 300 * 1024 ** 3, out: 100 * 1024 ** 3, used: 400 * 1024 ** 3, limit: 1024 ** 4, mode: 'sum', reset_day: 1, period: '2026-09-01' },
    expire_at: null,
    uptime_24h: 99.9,
    ipv4: true,
    ipv6: false,
    ...overrides,
  }
}

export function makeAdminServer(overrides: Partial<AdminServer> = {}): AdminServer {
  return {
    id: 'srv1',
    name: 'hk-01',
    secret: 'secret-1',
    group: 'HK',
    country: 'HK',
    sort: 0,
    hidden: false,
    price: null,
    currency: '',
    billing_cycle: '',
    expire_at: null,
    traffic_limit: null,
    traffic_mode: 'sum',
    traffic_reset_day: 1,
    report_interval: 2,
    nic_include: [],
    nic_exclude: [],
    mount_exclude: [],
    note: '',
    notify_muted: false,
    static_info: makeHello(),
    last_ip: '1.2.3.4',
    last_seen: 1_700_000_000,
    created_at: 1_700_000_000,
    updated_at: 1_700_000_000,
    online: true,
    agent_version: '2.0.0',
    outdated: false,
    uptime_24h: 99.9,
    ...overrides,
  }
}
