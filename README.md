# Simple Server Status

[![GitHub release](https://img.shields.io/github/release/ruanun/simple-server-status.svg)](https://github.com/ruanun/simple-server-status/releases)
[![CI](https://github.com/ruanun/simple-server-status/actions/workflows/ci.yml/badge.svg)](https://github.com/ruanun/simple-server-status/actions/workflows/ci.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/ruanun/sssd)](https://hub.docker.com/r/ruanun/sssd)
[![License](https://img.shields.io/github/license/ruanun/simple-server-status.svg)](LICENSE)

极简的多服务器探针：Dashboard 为单个 Go 程序加 SQLite 数据库，Agent 为单个 Go 程序，通过 WebSocket 上报。

## 特性

- 实时状态：CPU、内存、硬盘、网速、负载、连接数，每 2 秒推送到浏览器
- 历史趋势：实时 / 1h / 6h / 24h / 7d；在线率（24 小时 / 7 天）与当月每日流量图
- 月流量配额（可选入站、出站或合计，自定义重置日），到期日与价格
- 按分组、地区筛选，卡片与列表两种视图
- 公开状态页支持公告，单台服务器可设为仅登录可见
- 管理后台：增删改服务器、拖拽排序、费用汇总与到期列表、Agent 版本与升级入口、随时查看 Linux / Windows 一键安装命令
- 通知：离线 / 恢复、持续高负载 / 恢复、重启、公网 IP 变化、即将到期、月流量超阈值，支持 Webhook 与 Telegram
- 事件记录：离线、高负载、重启、公网 IP 变化，不配置通知也会记录，连同通知发送记录在后台「事件」页可查（详情页显示最近离线），保留 90 天
- IPv4 / IPv6 标记：状态页与后台显示服务器的公网协议栈支持情况
- 导入导出 JSON，便于整机迁移
- 亮 / 暗主题，中文 / 英文界面

## 快速开始

> 2.0 目前处于测试阶段：镜像请使用 `ruanun/sssd:beta`（`latest` 仍为 1.x），二进制与安装脚本见 GitHub Releases 中的预发布版本。

启动 Dashboard：

```bash
docker run -d --name sss-dashboard --restart unless-stopped \
  -p 8900:8900 -v sss-data:/app/data ruanun/sssd:latest
docker logs sss-dashboard 2>&1 | grep password   # 首次生成的 admin 密码
```

添加 Agent：

1. 打开 `http://<服务器IP>:8900/login`，用 `admin` 登录。
2. 在「服务器」页新建服务器，保存后弹出安装命令。
3. 复制 Linux 或 Windows 命令到目标机器执行，回到首页即可看到该服务器上线。

详细步骤见 [快速开始](docs/getting-started.md)。

## 文档

- [快速开始](docs/getting-started.md)：Docker 部署、首次登录、接入第一台服务器、重置密码
- [部署](docs/deployment.md)：Docker / systemd、反向代理、Agent 安装与卸载、服务管理、升级备份、从源码构建
- [配置](docs/configuration.md)：Dashboard 与 Agent 参数、后台设置、服务器字段
- [API](docs/api.md)：HTTP 接口、浏览器与 Agent 的 WebSocket 协议

## 本地开发

需要 Go 1.24+、Node.js 22+、pnpm 10。

```bash
make help          # 列出全部命令
make build         # 构建前端与两个程序，产物在 bin/
make check         # go vet、golangci-lint、Go 测试、前端类型检查 / lint / 单元测试
make e2e           # Playwright 端到端测试
```

前端开发时先启动本地 Dashboard，再启动 Vite 开发服务器（`/api` 与 WebSocket 代理到 `http://127.0.0.1:8900`，可用环境变量 `SSS_DASHBOARD_TARGET` 修改）：

```bash
make run-dashboard   # 终端 1
make dev-web         # 终端 2
```

Windows 没有 make 时的等价命令见 [部署 - 从源码构建](docs/deployment.md#从源码构建)。

## 技术栈

Go（gin、coder/websocket、modernc.org/sqlite、cobra、gopsutil）；React 19、TypeScript、Vite、Tailwind CSS v4、shadcn/ui、TanStack Query、Recharts、i18next。

## License

[MIT](LICENSE)
