/**
 * English translation
 * @author ruan
 */

const enUS = {
  serverInfo: {
    labels: {
      system: 'System',
      cpuUsage: 'CPU',
      memoryUsage: 'Memory',
      swapMemory: 'Swap',
      networkSpeed: 'Network',
      uptime: 'Uptime',
      lastUpdate: 'Updated',
      cpuInfo: 'CPU',
      memoryDetails: 'Memory',
      systemLoad: 'Load',
      totalTraffic: 'Traffic',
      diskUsage: 'Disk'
    },
    units: {
      percent: '%',
      mbps: 'MB/s',
      kbps: 'KB/s',
      bps: 'B/s',
      gb: 'GB',
      mb: 'MB',
      kb: 'KB',
      bytes: 'Bytes',
      days: 'd',
      hours: 'h',
      minutes: 'm',
      seconds: 's'
    },
    table: {
      mountPoint: 'Mount',
      used: 'Used',
      total: 'Total'
    },
    status: {
      online: 'Online',
      offline: 'Offline'
    }
  },
  common: {
    language: 'Language',
    switchTo: 'Switch to',
    settings: 'Settings'
  },
  header: {
    servers: 'Servers',
    online: 'Online',
    status: {
      connected: 'Connected',
      connecting: 'Connecting',
      reconnecting: 'Reconnecting',
      disconnected: 'Disconnected',
      error: 'Connection Error',
      unknown: 'Unknown Status',
      httpPolling: 'HTTP Polling'
    },
    mode: {
      websocket: 'WebSocket',
      http: 'HTTP'
    },
    actions: {
      switchToHttp: 'Switch to HTTP Polling Mode',
      switchToWebsocket: 'Switch to WebSocket Real-time Mode',
      reconnect: 'Reconnect WebSocket'
    },
    stats: {
      title: 'Connection Stats',
      messages: 'Messages',
      reconnections: 'Reconnections',
      uptime: 'Uptime',
      connectedSince: 'Connected since'
    }
  }
}

export default enUS
