# 前端构建脚本（Windows PowerShell 版本）
# 作者: ruan
# 说明: 构建前端项目并复制到 embed 目录

$ErrorActionPreference = "Stop"

# 获取项目根目录
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir

Write-Host "📦 开始构建前端项目..." -ForegroundColor Green

# 检查 Node.js 是否安装
try {
    $nodeVersion = node --version
    Write-Host "✓ Node.js 版本: $nodeVersion" -ForegroundColor Green
} catch {
    Write-Host "❌ 错误: 未找到 Node.js" -ForegroundColor Red
    Write-Host "请先安装 Node.js: https://nodejs.org/" -ForegroundColor Yellow
    exit 1
}

# 检查 pnpm 是否安装
try {
    $pnpmVersion = pnpm --version
    Write-Host "✓ pnpm 版本: $pnpmVersion" -ForegroundColor Green
} catch {
    Write-Host "❌ 错误: 未找到 pnpm" -ForegroundColor Red
    Write-Host "请先安装 pnpm: npm install -g pnpm 或 corepack enable" -ForegroundColor Yellow
    exit 1
}

# 进入 web 目录
$WebDir = Join-Path $ProjectRoot "web"
Set-Location $WebDir

# 检查 package.json 是否存在
if (-Not (Test-Path "package.json")) {
    Write-Host "❌ 错误: 未找到 package.json" -ForegroundColor Red
    exit 1
}

# 安装依赖（仅在 node_modules 不存在时）
if (-Not (Test-Path "node_modules")) {
    Write-Host "📥 安装前端依赖..." -ForegroundColor Yellow
    pnpm install --frozen-lockfile
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ 依赖安装失败" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "✓ 依赖已存在，跳过安装" -ForegroundColor Green
}

# 构建前端项目
Write-Host "🔨 构建前端项目（生产模式）..." -ForegroundColor Yellow
pnpm run build:prod
if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ 构建失败" -ForegroundColor Red
    exit 1
}

# 检查构建产物是否存在
$DistDir = Join-Path $WebDir "dist"
if (-Not (Test-Path $DistDir)) {
    Write-Host "❌ 错误: 构建失败，未找到 dist 目录" -ForegroundColor Red
    exit 1
}

# 返回项目根目录
Set-Location $ProjectRoot

# 目标目录
$EmbedDir = Join-Path $ProjectRoot "internal\dashboard\public\dist"

# 创建目标目录（如果不存在）
if (-Not (Test-Path $EmbedDir)) {
    New-Item -ItemType Directory -Path $EmbedDir -Force | Out-Null
}

# 清空目标目录（保留 .gitkeep 或 README.md）
Write-Host "🗑️  清理 embed 目录..." -ForegroundColor Yellow
Get-ChildItem -Path $EmbedDir -Recurse |
    Where-Object { $_.Name -ne '.gitkeep' -and $_.Name -ne 'README.md' } |
    Remove-Item -Recurse -Force

# 复制构建产物
Write-Host "📋 复制构建产物到 embed 目录..." -ForegroundColor Yellow
Copy-Item -Path "$DistDir\*" -Destination $EmbedDir -Recurse -Force

# 验证复制结果
$AssetsDir = Join-Path $EmbedDir "assets"
if (Test-Path $AssetsDir) {
    Write-Host "✅ 前端构建完成！" -ForegroundColor Green
    Write-Host "   输出目录: $EmbedDir" -ForegroundColor Green

    # 显示文件统计
    $FileCount = (Get-ChildItem -Path $EmbedDir -Recurse -File).Count
    Write-Host "   文件数量: $FileCount" -ForegroundColor Green
} else {
    Write-Host "❌ 错误: 复制失败，未找到 assets 目录" -ForegroundColor Red
    exit 1
}
