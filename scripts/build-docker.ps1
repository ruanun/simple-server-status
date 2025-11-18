# ============================================
# Docker 本地构建测试脚本 (PowerShell 版本)
# 作者: ruan
# 用途: 在 Windows 本地测试 Docker 镜像构建
# ============================================

$ErrorActionPreference = "Stop"

# 配置变量
$ImageName = "sssd"
$Tag = "dev"
$Version = "dev-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
$Commit = & git rev-parse --short HEAD 2>$null
if (-not $Commit) { $Commit = "unknown" }
$BuildDate = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

Write-Host "========================================" -ForegroundColor Green
Write-Host "Docker 本地构建测试" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
Write-Host "镜像信息:" -ForegroundColor Yellow
Write-Host "  名称: $ImageName"
Write-Host "  标签: $Tag"
Write-Host "  版本: $Version"
Write-Host "  提交: $Commit"
Write-Host "  构建时间: $BuildDate"
Write-Host ""

# 检查 Docker 是否安装
try {
    $null = docker --version
} catch {
    Write-Host "错误: Docker 未安装" -ForegroundColor Red
    exit 1
}

# 构建选项
$BuildPlatform = "linux/amd64"
$MultiArch = $args -contains "--multi-arch"

if ($MultiArch) {
    $BuildPlatform = "linux/amd64,linux/arm64,linux/arm/v7"
    Write-Host "多架构构建模式: $BuildPlatform" -ForegroundColor Yellow

    # 检查 buildx 是否可用
    try {
        $null = docker buildx version
    } catch {
        Write-Host "错误: Docker Buildx 未安装" -ForegroundColor Red
        Write-Host "请运行: docker buildx install"
        exit 1
    }
} else {
    Write-Host "单架构构建模式: $BuildPlatform" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "开始构建 Docker 镜像..." -ForegroundColor Green
Write-Host ""

# 构建镜像
try {
    if ($MultiArch) {
        # 多架构构建
        docker buildx build `
            --platform $BuildPlatform `
            --build-arg VERSION="$Version" `
            --build-arg COMMIT="$Commit" `
            --build-arg BUILD_DATE="$BuildDate" `
            --build-arg TZ="Asia/Shanghai" `
            -t ${ImageName}:${Tag} `
            -f Dockerfile `
            --load `
            .
    } else {
        # 单架构构建
        docker build `
            --platform $BuildPlatform `
            --build-arg VERSION="$Version" `
            --build-arg COMMIT="$Commit" `
            --build-arg BUILD_DATE="$BuildDate" `
            --build-arg TZ="Asia/Shanghai" `
            -t ${ImageName}:${Tag} `
            -f Dockerfile `
            .
    }

    Write-Host ""
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "构建成功！" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host ""

    # 显示镜像信息
    Write-Host "镜像详情:" -ForegroundColor Yellow
    docker images ${ImageName}:${Tag}
    Write-Host ""

    Write-Host "镜像大小分析:" -ForegroundColor Yellow
    $imageSize = docker image inspect ${ImageName}:${Tag} --format='{{.Size}}'
    $imageSizeMB = [math]::Round($imageSize / 1MB, 2)
    Write-Host "镜像大小: $imageSize bytes ($imageSizeMB MB)"
    Write-Host ""

    # 提供运行命令
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "测试运行命令:" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "1. 使用示例配置运行:"
    Write-Host "   docker run --rm -p 8900:8900 -v `$(pwd)/configs/sss-dashboard.yaml.example:/app/sss-dashboard.yaml ${ImageName}:${Tag}" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "2. 交互式运行（调试）:"
    Write-Host "   docker run --rm -it -p 8900:8900 ${ImageName}:${Tag} sh" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "3. 后台运行:"
    Write-Host "   docker run -d --name sssd-test -p 8900:8900 -v `$(pwd)/configs/sss-dashboard.yaml.example:/app/sss-dashboard.yaml ${ImageName}:${Tag}" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "4. 查看日志:"
    Write-Host "   docker logs -f sssd-test" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "5. 停止并删除容器:"
    Write-Host "   docker stop sssd-test; docker rm sssd-test" -ForegroundColor Yellow
    Write-Host ""

} catch {
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Red
    Write-Host "构建失败！" -ForegroundColor Red
    Write-Host "========================================" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    exit 1
}
