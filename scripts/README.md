# 脚本使用说明

本目录包含 Simple Server Status 的自动化脚本，包括安装脚本和构建脚本。

**项目地址：** https://github.com/ruanun/simple-server-status
**演示地址：** https://sssd.ions.top/

## 📋 脚本列表

### 安装脚本

| 脚本文件 | 支持系统 | 功能描述 |
|----------|----------|----------|
| `install-agent.sh` | Linux, macOS, FreeBSD | Unix 系统一键安装脚本 |
| `install-agent.ps1` | Windows | Windows PowerShell 安装脚本 |

**详细使用说明：** 参考 [完整部署指南](../docs/getting-started.md)

### 构建脚本

| 脚本文件 | 支持系统 | 功能描述 |
|----------|----------|----------|
| `build-web.sh` | Linux, macOS, FreeBSD | Unix 系统前端构建脚本 |
| `build-web.ps1` | Windows | Windows PowerShell 前端构建脚本 |
| `build-dashboard.sh` | Linux, macOS, FreeBSD | Dashboard 完整构建脚本（含前端） |

---

# 📦 构建脚本使用指南

## 概述

构建脚本用于自动化前端构建和 Dashboard 编译流程，解决手动复制前端产物到 embed 目录的问题。

### 工作原理

Dashboard 使用 Go 的 `embed.FS` 将前端文件嵌入到可执行文件中：

```go
// internal/dashboard/public/resource.go
//go:embed dist
var Resource embed.FS
```

构建脚本自动完成以下流程：
1. 构建前端项目（`web/` → `web/dist/`）
2. 复制产物到 embed 目录（`web/dist/` → `internal/dashboard/public/dist/`）
3. 构建 Dashboard Go 程序（嵌入前端文件）

## 🚀 快速使用

### 方式一：使用 Makefile（推荐）

```bash
# 只构建前端
make build-web

# 构建完整的 Dashboard（自动包含前端）
make build-dashboard

# 只构建 Dashboard（跳过前端，需要前端已构建）
make build-dashboard-only

# 启动前端开发服务器
make dev-web

# 清理所有产物
make clean

# 只清理前端产物
make clean-web
```

### 方式二：直接运行脚本

**Unix 系统（Linux/macOS/FreeBSD）：**

```bash
# 设置执行权限（首次需要）
chmod +x scripts/*.sh

# 只构建前端
bash scripts/build-web.sh

# 构建完整的 Dashboard
bash scripts/build-dashboard.sh
```

**Windows 系统：**

```powershell
# 构建前端
powershell -File scripts/build-web.ps1

# 注意：Windows 暂无完整的 Dashboard 构建脚本
# 请使用以下方式：
powershell -File scripts/build-web.ps1
go build -o bin/sss-dashboard.exe ./cmd/dashboard
```

## 📖 构建脚本详细说明

### build-web.sh / build-web.ps1

**功能：** 构建前端项目并复制到 embed 目录

**执行步骤：**
1. ✅ 检查 Node.js 和 pnpm 是否安装
2. ✅ 显示 Node.js 和 pnpm 版本信息
3. ✅ 进入 `web/` 目录
4. ✅ 安装依赖（如果 `node_modules` 不存在）
5. ✅ 执行生产构建：`pnpm run build:prod`
6. ✅ 清理目标目录（保留 `.gitkeep` 和 `README.md`）
7. ✅ 复制构建产物到 `internal/dashboard/public/dist/`
8. ✅ 验证复制结果并显示统计信息

**输出示例：**

```
📦 开始构建前端项目...
✓ Node.js 版本: v20.10.0
✓ pnpm 版本: 10.2.3
✓ 依赖已存在，跳过安装
🔨 构建前端项目（生产模式）...
🗑️  清理 embed 目录...
📋 复制构建产物到 embed 目录...
✅ 前端构建完成！
   输出目录: /path/to/internal/dashboard/public/dist
   文件数量: 15
```

**错误处理：**
- ❌ 未安装 Node.js → 提示安装链接并退出
- ❌ 未安装 pnpm → 提示错误并退出
- ❌ 未找到 package.json → 提示错误并退出
- ❌ 构建失败 → 提示错误并退出
- ❌ 复制失败 → 提示错误并退出

### build-dashboard.sh

**功能：** 构建完整的 Dashboard（包含前端和后端）

**执行步骤：**
1. ✅ 调用 `build-web.sh` 构建前端
2. ✅ 检查 Go 是否安装
3. ✅ 显示 Go 版本信息
4. ✅ 创建 `bin/` 目录
5. ✅ 编译 Dashboard：`go build -v -o bin/sss-dashboard ./cmd/dashboard`
6. ✅ 设置可执行权限
7. ✅ 显示构建结果和文件大小

**输出示例：**

```
================================
  Dashboard 完整构建流程
================================

📦 步骤 1/2: 构建前端项目

[前端构建输出...]

✓ 前端构建完成

🔧 步骤 2/2: 构建 Dashboard 二进制文件

✓ Go 版本: go version go1.23.2 linux/amd64
🔨 编译 Dashboard...
✅ Dashboard 构建成功！

================================
  构建完成
================================
  二进制文件: /path/to/bin/sss-dashboard
  文件大小: 25M

运行方式:
  ./bin/sss-dashboard
```

## 🔧 CI/CD 集成

构建脚本已集成到 GitHub Actions 工作流中。

### CI 构建流程（.github/workflows/ci.yml）

**Unix 系统（Ubuntu/macOS）：**
```yaml
- name: 构建前端（Unix 系统）
  if: matrix.os != 'windows-latest'
  run: bash scripts/build-web.sh

- name: 构建 Dashboard
  run: go build -v -o bin/sss-dashboard ./cmd/dashboard
```

**Windows 系统：**
```yaml
- name: 构建前端（Windows 系统）
  if: matrix.os == 'windows-latest'
  run: powershell -File scripts/build-web.ps1

- name: 构建 Dashboard
  run: go build -v -o bin/sss-dashboard.exe ./cmd/dashboard
```

### Release 构建流程（.github/workflows/release.yml）

```yaml
- name: 构建前端
  run: bash scripts/build-web.sh

- name: 运行 GoReleaser
  uses: goreleaser/goreleaser-action@v5
  with:
    args: release --clean
```

## 🛠️ 开发工作流

### 日常开发

1. **前端开发**
   ```bash
   # 启动前端开发服务器（热重载）
   make dev-web
   # 或
   cd web && npm run dev
   ```

2. **后端开发**
   ```bash
   # 只构建后端（使用已有的前端产物）
   make build-dashboard-only

   # 运行 Dashboard
   ./bin/sss-dashboard
   ```

3. **完整测试**
   ```bash
   # 重新构建前端和后端
   make build-dashboard

   # 运行
   ./bin/sss-dashboard
   ```

### 发布版本

1. **本地测试构建**
   ```bash
   # 构建前端
   make build-web

   # 使用 GoReleaser 构建多平台版本
   make release
   # 或
   goreleaser release --snapshot --clean
   ```

2. **推送标签触发自动发布**
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   # GitHub Actions 自动构建和发布
   ```

## 🐛 故障排除

### 问题 1: 权限被拒绝

**错误：** `Permission denied: scripts/build-web.sh`

**解决：**
```bash
chmod +x scripts/*.sh
```

### 问题 2: Node.js 或 pnpm 未找到

**错误：** `command not found: node` 或 `command not found: pnpm`

**解决：**
```bash
# 安装 Node.js
# Ubuntu/Debian
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs

# macOS
brew install node

# 或使用 nvm
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.0/install.sh | bash
nvm install 20

# 安装 pnpm
npm install -g pnpm
# 或
curl -fsSL https://get.pnpm.io/install.sh | sh -
```

### 问题 3: 构建产物未找到

**错误：** `未找到 assets 目录`

**原因：** 前端构建失败或配置错误

**解决：**
1. 检查 `web/package.json` 中的 `build:prod` 脚本
2. 确认 Vite 配置正确
3. 手动测试前端构建：
   ```bash
   cd web
   pnpm install --frozen-lockfile
   pnpm run build:prod
   ls dist  # 应该看到 index.html 和 assets 目录
   ```

### 问题 4: Go 版本过低

**错误：** `go version go1.20 is too old`

**解决：**
```bash
# 下载并安装 Go 1.23+
wget https://go.dev/dl/go1.23.2.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.2.linux-amd64.tar.gz
```

### 问题 5: Windows PowerShell 执行策略

**错误：** `无法加载脚本`

**解决：**
```powershell
# 临时允许（推荐）
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
powershell -File scripts/build-web.ps1

# 或永久允许（需要管理员权限）
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

## 📚 参考资料

- [Go embed 包文档](https://pkg.go.dev/embed)
- [Vite 构建文档](https://vitejs.dev/guide/build.html)
- [Makefile 教程](https://makefiletutorial.com/)

---

# 📥 安装脚本快速使用

## 🚀 快速安装

### Linux/macOS/FreeBSD

```bash
# 在线安装（推荐）
curl -fsSL https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.sh | sudo bash
```

### Windows

```powershell
# 在线安装（推荐）
iwr -useb https://raw.githubusercontent.com/ruanun/simple-server-status/main/scripts/install-agent.ps1 | iex
```

## 🔧 命令行参数

### install-agent.sh 参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `--version <版本>` | 指定安装版本 | `--version v1.2.0` |
| `--install-dir <目录>` | 自定义安装目录 | `--install-dir /opt/sssa` |
| `--uninstall` | 卸载 Agent | `--uninstall` |
| `--help` | 显示帮助信息 | `--help` |

### install-agent.ps1 参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `-Version <版本>` | 指定安装版本 | `-Version "v1.2.0"` |
| `-InstallDir <目录>` | 自定义安装目录 | `-InstallDir "D:\SSSA"` |
| `-Uninstall` | 卸载 Agent | `-Uninstall` |
| `-Help` | 显示帮助信息 | `-Help` |

## 📖 详细文档

更多安装选项、故障排除和高级配置，请参考：

- 📥 **[完整部署指南](../docs/getting-started.md)** - 详细的安装和配置步骤
- 🔧 **[手动安装](../docs/deployment/manual.md)** - 不使用脚本的手动安装方法
- 🐛 **[故障排除指南](../docs/troubleshooting.md)** - 常见问题和解决方案
- 🔄 **[维护指南](../docs/maintenance.md)** - 更新、备份和卸载

---

> 💡 **提示**: 脚本会自动检测系统环境并选择最佳的安装方式。如果遇到问题，请查看详细的安装指南或提交 Issue。