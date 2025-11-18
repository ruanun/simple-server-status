/**
 * 简体中文翻译
 * @author ruan
 */

const zhCN = {
  serverInfo: {
    labels: {
      system: '系统',
      cpuUsage: 'CPU',
      memoryUsage: '内存',
      swapMemory: '交换区',
      networkSpeed: '网络',
      uptime: '运行时间',
      lastUpdate: '最后更新',
      cpuInfo: 'CPU',
      memoryDetails: '内存',
      systemLoad: '负载',
      totalTraffic: '流量',
      diskUsage: '磁盘'
    },
    units: {
      percent: '%',
      mbps: 'MB/s',
      kbps: 'KB/s',
      bps: 'B/s',
      gb: 'GB',
      mb: 'MB',
      kb: 'KB',
      bytes: '字节',
      days: '天',
      hours: '小时',
      minutes: '分钟',
      seconds: '秒'
    },
    table: {
      mountPoint: '挂载点',
      used: '已用',
      total: '总计'
    },
    status: {
      online: '在线',
      offline: '离线'
    }
  },
  common: {
    language: '语言',
    switchTo: '切换到',
    settings: '设置'
  },
  header: {
    servers: '服务器',
    online: '在线',
    status: {
      connected: '已连接',
      connecting: '连接中',
      reconnecting: '重连中',
      disconnected: '已断开',
      error: '连接错误',
      unknown: '未知状态',
      httpPolling: 'HTTP 轮询'
    },
    mode: {
      websocket: 'WebSocket',
      http: 'HTTP'
    },
    actions: {
      switchToHttp: '切换到 HTTP 轮询模式',
      switchToWebsocket: '切换到 WebSocket 实时模式',
      reconnect: '重新连接 WebSocket'
    },
    stats: {
      title: '连接统计',
      messages: '消息',
      reconnections: '重连',
      uptime: '连接时长',
      connectedSince: '连接于'
    }
  }
}

export default zhCN
