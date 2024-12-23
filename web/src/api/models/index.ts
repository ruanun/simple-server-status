export interface ServerInfo {
    name: string;
    group: string;
    id: string;
    lastReportTime: number;
    uptime: number;
    platform: string;

    cpuPercent: number;
    RAMPercent: number;
    SWAPPercent: number;
    diskPercent: number;
    netInSpeed: number;
    netOutSpeed: number;

    isOnline: boolean

    hostInfo: HostInfo;

    loc: string;
}

export interface HostInfo {
    cpuInfo: string[];
    avgStat: AvgStat;
    RAMTotal: number;
    RAMUsed: number;
    swapTotal: number;
    swapUsed: number;
    diskTotal: number;
    diskUsed: number;
    diskPartitions: DiskPartition[];
    netInTransfer: number;
    netOutTransfer: number;
}

export interface DiskPartition {
    mountPoint: string;
    fstype: string;
    total: number;
    free: number;
    used: number;
    usedPercent: number;
}

export interface AvgStat {
    load1: number;
    load5: number;
    load15: number;
}