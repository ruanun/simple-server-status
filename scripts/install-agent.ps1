# Simple Server Status Agent Windows 安装脚本
# PowerShell 脚本，支持 Windows 系统

param(
    [switch]$Uninstall,
    [switch]$Help,
    [string]$Version = "",
    [string]$InstallDir = "C:\Program Files\SSSA"
)

# 项目信息
$REPO = if ($env:REPO) { $env:REPO } else { "ruanun/simple-server-status" }
$BINARY_NAME = "sss-agent.exe"
$SERVICE_NAME = "SSSA"
$CONFIG_FILE = "sss-agent.yaml"

# 函数：打印彩色信息
function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Blue
}

function Write-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# 函数：检查管理员权限
function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# 函数：检查管理员权限
function Assert-Administrator {
    if (-not (Test-Administrator)) {
        Write-Error "此脚本需要管理员权限运行"
        Write-Info "请以管理员身份运行 PowerShell"
        exit 1
    }
}

# 函数：检测系统架构
function Get-SystemArchitecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        "x86" { return "386" }
        default {
            Write-Error "不支持的架构: $arch"
            exit 1
        }
    }
}

# 函数：获取最新版本
function Get-LatestVersion {
    Write-Info "获取最新版本信息..."

    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$REPO/releases/latest" -Method Get
        $version = $response.tag_name

        if ([string]::IsNullOrEmpty($version)) {
            throw "无法获取版本信息"
        }

        Write-Info "最新版本: $version"
        return $version
    }
    catch {
        Write-Error "无法获取最新版本信息: $($_.Exception.Message)"
        exit 1
    }
}

# 函数：下载文件
function Download-File {
    param(
        [string]$Url,
        [string]$OutputPath
    )

    Write-Info "下载: $Url"

    try {
        # 创建目录（如果不存在）
        $dir = Split-Path $OutputPath -Parent
        if (!(Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir -Force | Out-Null
        }

        # 下载文件
        Invoke-WebRequest -Uri $Url -OutFile $OutputPath -UseBasicParsing
        Write-Success "下载完成: $OutputPath"
    }
    catch {
        Write-Error "下载失败: $($_.Exception.Message)"
        exit 1
    }
}

# 函数：解压ZIP文件
function Expand-ZipFile {
    param(
        [string]$ZipPath,
        [string]$ExtractPath
    )

    Write-Info "解压文件: $ZipPath"

    try {
        # 确保目标目录存在
        if (!(Test-Path $ExtractPath)) {
            New-Item -ItemType Directory -Path $ExtractPath -Force | Out-Null
        }

        # 解压文件
        Expand-Archive -Path $ZipPath -DestinationPath $ExtractPath -Force
        Write-Success "解压完成"
    }
    catch {
        Write-Error "解压失败: $($_.Exception.Message)"
        exit 1
    }
}

# 函数：安装Windows服务
function Install-WindowsService {
    param(
        [string]$ServicePath,
        [string]$ConfigPath
    )

    Write-Info "安装Windows服务..."

    try {
        # 检查服务是否已存在
        $existingService = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
        if ($existingService) {
            Write-Info "服务已存在，先停止并删除..."
            Stop-Service -Name $SERVICE_NAME -Force -ErrorAction SilentlyContinue
            & sc.exe delete $SERVICE_NAME
            Start-Sleep -Seconds 2
        }

        # 创建服务
        $serviceBinary = "`"$ServicePath`" -c `"$ConfigPath`""
        & sc.exe create $SERVICE_NAME binPath= $serviceBinary start= auto DisplayName= "Simple Server Status Agent"

        if ($LASTEXITCODE -eq 0) {
            Write-Success "Windows服务安装成功"
            Write-Info "服务管理命令:"
            Write-Info "  启动服务: Start-Service -Name $SERVICE_NAME"
            Write-Info "  停止服务: Stop-Service -Name $SERVICE_NAME"
            Write-Info "  查看状态: Get-Service -Name $SERVICE_NAME"
        } else {
            Write-Warning "服务安装失败，可以手动运行程序"
        }
    }
    catch {
        Write-Warning "服务安装失败: $($_.Exception.Message)"
        Write-Info "可以手动运行程序"
    }
}

# 函数：下载并安装
function Install-Agent {
    $arch = Get-SystemArchitecture
    Write-Info "检测到系统架构: $arch"

    # 获取版本
    if ([string]::IsNullOrEmpty($Version)) {
        $Version = Get-LatestVersion
    } else {
        Write-Info "使用指定版本: $Version"
    }

    # 构建下载URL（与 GoReleaser 格式一致）
    $archiveName = "sss-agent_${Version}_windows_${arch}.zip"
    $downloadUrl = "https://github.com/$REPO/releases/download/$Version/$archiveName"

    # 创建临时目录
    $tempDir = Join-Path $env:TEMP "sssa-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    try {
        # 下载文件
        $zipPath = Join-Path $tempDir $archiveName
        Download-File -Url $downloadUrl -OutputPath $zipPath

        # 解压文件
        $extractPath = Join-Path $tempDir "extract"
        Expand-ZipFile -ZipPath $zipPath -ExtractPath $extractPath

        # 查找解压后的目录
        $extractedDir = Get-ChildItem -Path $extractPath -Directory | Where-Object { $_.Name -like "sss-agent*" } | Select-Object -First 1
        if (-not $extractedDir) {
            Write-Error "无法找到解压后的目录"
            exit 1
        }

        $sourceDir = $extractedDir.FullName

        # 创建安装目录
        Write-Info "创建安装目录: $InstallDir"
        if (!(Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }

        # 复制二进制文件
        Write-Info "安装二进制文件..."
        $sourceBinary = Join-Path $sourceDir $BINARY_NAME
        $targetBinary = Join-Path $InstallDir $BINARY_NAME

        if (Test-Path $sourceBinary) {
            Copy-Item -Path $sourceBinary -Destination $targetBinary -Force
            Write-Success "二进制文件安装完成"
        } else {
            Write-Error "找不到二进制文件: $sourceBinary"
            exit 1
        }

        # 复制配置文件示例
        $sourceConfig = Join-Path $sourceDir "configs/sss-agent.yaml.example"
        $targetConfig = Join-Path $InstallDir $CONFIG_FILE

        if ((Test-Path $sourceConfig) -and !(Test-Path $targetConfig)) {
            Write-Info "复制配置文件示例..."
            Copy-Item -Path $sourceConfig -Destination $targetConfig -Force
            Write-Warning "请编辑配置文件: $targetConfig"
        } elseif (Test-Path $targetConfig) {
            Write-Info "配置文件已存在，跳过复制"
        }

        # 添加到系统PATH
        Write-Info "添加到系统PATH..."
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($currentPath -notlike "*$InstallDir*") {
            $newPath = "$currentPath;$InstallDir"
            [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
            Write-Success "已添加到系统PATH"
        } else {
            Write-Info "已在系统PATH中"
        }

        # 安装Windows服务
        Install-WindowsService -ServicePath $targetBinary -ConfigPath $targetConfig

        Write-Success "安装完成！"
    }
    finally {
        # 清理临时文件
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# 函数：卸载
function Uninstall-Agent {
    Write-Info "开始卸载 Simple Server Status Agent..."

    # 停止并删除服务
    $service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
    if ($service) {
        Write-Info "停止并删除Windows服务..."
        Stop-Service -Name $SERVICE_NAME -Force -ErrorAction SilentlyContinue
        & sc.exe delete $SERVICE_NAME
        Write-Success "服务已删除"
    }

    # 从PATH中移除
    Write-Info "从系统PATH中移除..."
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -like "*$InstallDir*") {
        $newPath = $currentPath -replace [regex]::Escape(";$InstallDir"), "" -replace [regex]::Escape("$InstallDir;"), ""
        [Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
        Write-Success "已从系统PATH中移除"
    }

    # 删除安装目录
    if (Test-Path $InstallDir) {
        $response = Read-Host "是否删除安装目录 $InstallDir ? (配置文件将被删除) [y/N]"
        if ($response -eq 'y' -or $response -eq 'Y') {
            Remove-Item -Path $InstallDir -Recurse -Force
            Write-Success "安装目录已删除"
        } else {
            $binaryPath = Join-Path $InstallDir $BINARY_NAME
            if (Test-Path $binaryPath) {
                Remove-Item -Path $binaryPath -Force
            }
            Write-Info "仅删除二进制文件，配置文件已保留"
        }
    }

    Write-Success "卸载完成！"
}

# 函数：显示使用说明
function Show-Usage {
    Write-Info "安装完成后的使用说明:"
    Write-Host ""
    Write-Info "1. 编辑配置文件:"
    Write-Host "   notepad `"$InstallDir\$CONFIG_FILE`""
    Write-Host ""
    Write-Info "2. 配置说明:"
    Write-Host "   - serverAddr: Dashboard服务器WebSocket地址"
    Write-Host "   - serverId: 服务器ID (在Dashboard中配置)"
    Write-Host "   - authSecret: 认证密钥 (与Dashboard配置一致)"
    Write-Host ""
    Write-Info "3. 启动服务:"
    Write-Host "   Start-Service -Name $SERVICE_NAME"
    Write-Host ""
    Write-Info "4. 查看服务状态:"
    Write-Host "   Get-Service -Name $SERVICE_NAME"
    Write-Host ""
    Write-Info "5. 手动运行 (如果服务安装失败):"
    Write-Host "   & `"$InstallDir\$BINARY_NAME`" -c `"$InstallDir\$CONFIG_FILE`""
    Write-Host ""
    Write-Info "6. 验证安装:"
    Write-Host "   sss-agent --version"
    Write-Host ""
}

# 函数：显示帮助
function Show-Help {
    Write-Host "Simple Server Status Agent Windows 安装脚本"
    Write-Host "============================================="
    Write-Host ""
    Write-Host "用法: .\install-agent.ps1 [选项]"
    Write-Host ""
    Write-Host "选项:"
    Write-Host "  -Uninstall         卸载 Simple Server Status Agent"
    Write-Host "  -Help              显示此帮助信息"
    Write-Host "  -Version <版本>    指定要安装的版本 (默认: 最新版本)"
    Write-Host "  -InstallDir <路径> 指定安装目录 (默认: C:\Program Files\SSSA)"
    Write-Host ""
    Write-Host "示例:"
    Write-Host "  .\install-agent.ps1                           # 安装最新版本"
    Write-Host "  .\install-agent.ps1 -Version v1.0.0          # 安装指定版本"
    Write-Host "  .\install-agent.ps1 -InstallDir C:\SSSA      # 安装到指定目录"
    Write-Host "  .\install-agent.ps1 -Uninstall               # 卸载"
    Write-Host ""
}

# 主函数
function Main {
    Write-Host "Simple Server Status Agent Windows 安装脚本" -ForegroundColor Cyan
    Write-Host "============================================" -ForegroundColor Cyan
    Write-Host ""

    # 处理参数
    if ($Help) {
        Show-Help
        return
    }

    if ($Uninstall) {
        Assert-Administrator
        Uninstall-Agent
        return
    }

    # 默认安装
    Assert-Administrator
    Install-Agent
    Show-Usage
}

# 执行主函数
Main
