# Simple Server Status Agent 安装脚本（Windows 服务）
# 作者: ruan
# 用法（管理员 PowerShell）:
#   Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force; $f="$env:TEMP\install-agent.ps1"; iwr -useb <脚本地址> -OutFile $f; & $f -Dashboard <地址> -Id <ID> -Secret <密钥>
#   & $f -Uninstall
param(
    [string]$Dashboard,
    [string]$Id,
    [string]$Secret,
    [string]$Version = 'latest',
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'
$Repo = 'ruanun/simple-server-status'
$ServiceName = 'sss-agent'
$InstallDir = Join-Path $env:ProgramFiles 'sss-agent'
$BinPath = Join-Path $InstallDir 'sss-agent.exe'
$ConfigPath = Join-Path $InstallDir 'sss-agent.yaml'
$LogPath = Join-Path $InstallDir 'logs\agent.log'

# Get-DownloadUrl 返回指定版本与架构的 Agent 压缩包地址
function Get-DownloadUrl([string]$Ver, [string]$Arch) {
    $file = "sss-agent_windows_$Arch.zip"
    if ($Ver -eq 'latest') { return "https://github.com/$Repo/releases/latest/download/$file" }
    return "https://github.com/$Repo/releases/download/$Ver/$file"
}

# ConvertTo-YamlQuoted 用 YAML 单引号包裹字符串，内部单引号写成两个
function ConvertTo-YamlQuoted([string]$Value) {
    return "'" + $Value.Replace("'", "''") + "'"
}

function Get-AgentConfig([string]$DashboardUrl, [string]$ServerId, [string]$ServerSecret, [string]$LogFile) {
    return @(
        '# Simple Server Status Agent 配置（由安装脚本生成）'
        "dashboard: $(ConvertTo-YamlQuoted $DashboardUrl)"
        "id: $(ConvertTo-YamlQuoted $ServerId)"
        "secret: $(ConvertTo-YamlQuoted $ServerSecret)"
        'detect_country: true'
        'log_level: info'
        "log_file: $(ConvertTo-YamlQuoted $LogFile)"
    ) -join "`n"
}

# Assert-InstallArgs 校验必填参数，返回去掉末尾斜杠的 Dashboard 地址
function Assert-InstallArgs([string]$DashboardUrl, [string]$ServerId, [string]$ServerSecret) {
    if (-not $DashboardUrl) { throw '缺少 -Dashboard' }
    if (-not $ServerId) { throw '缺少 -Id' }
    if (-not $ServerSecret) { throw '缺少 -Secret' }
    if ($DashboardUrl -notmatch '^https?://') { throw 'Dashboard 地址需以 http:// 或 https:// 开头' }
    return $DashboardUrl.TrimEnd('/')
}

function Assert-Admin {
    $principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw '请在管理员 PowerShell 中运行'
    }
}

# Remove-AgentService 停止并删除已有服务，等待服务管理器完成删除
function Remove-AgentService {
    if (-not (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue)) { return }
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $ServiceName | Out-Null
    for ($i = 0; $i -lt 20 -and (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue); $i++) {
        Start-Sleep -Milliseconds 500
    }
}

function Install-Agent {
    $dash = Assert-InstallArgs $Dashboard $Id $Secret
    Assert-Admin
    if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') { throw "不支持的架构: $env:PROCESSOR_ARCHITECTURE" }

    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $ProgressPreference = 'SilentlyContinue'
    $url = Get-DownloadUrl $Version 'amd64'
    $tmp = Join-Path ([IO.Path]::GetTempPath()) ("sss-agent-" + [guid]::NewGuid())
    New-Item -ItemType Directory -Path $tmp | Out-Null
    try {
        Write-Host "下载 Agent：$url"
        Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile (Join-Path $tmp 'agent.zip')
        Expand-Archive -Path (Join-Path $tmp 'agent.zip') -DestinationPath $tmp -Force

        # 已安装时先删除旧服务，再覆盖程序与配置
        Remove-AgentService
        New-Item -ItemType Directory -Path (Split-Path $LogPath) -Force | Out-Null
        Copy-Item (Join-Path $tmp 'sss-agent.exe') $BinPath -Force
    } finally {
        Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
    }

    [IO.File]::WriteAllText($ConfigPath, (Get-AgentConfig $dash $Id $Secret $LogPath), (New-Object Text.UTF8Encoding $false))
    # 配置含密钥：仅 SYSTEM（S-1-5-18）与 Administrators（S-1-5-32-544）可访问，用 SID 避免中文系统下组名不同
    icacls $ConfigPath /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null

    New-Service -Name $ServiceName -DisplayName 'Simple Server Status Agent' -StartupType Automatic `
        -BinaryPathName "`"$BinPath`" --config `"$ConfigPath`"" | Out-Null
    # 异常退出（含非 0 退出码）时 5 秒后自动重启；Dashboard 要求停止时以 0 退出，不会被重启
    sc.exe failure $ServiceName reset= 86400 actions= restart/5000/restart/5000/restart/5000 | Out-Null
    sc.exe failureflag $ServiceName 1 | Out-Null
    Start-Service -Name $ServiceName

    Start-Sleep -Seconds 2
    if ((Get-Service -Name $ServiceName).Status -ne 'Running') {
        throw "Agent 未能启动，请查看日志：$LogPath"
    }
    Write-Host '安装完成，Agent 已启动' -ForegroundColor Green
    Write-Host "  查看状态: Get-Service $ServiceName"
    Write-Host "  查看日志: Get-Content '$LogPath' -Tail 50"
    Write-Host '  卸载:     & $f -Uninstall'
}

function Uninstall-Agent {
    Assert-Admin
    Remove-AgentService
    Remove-Item $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
    # Agent 启动失败时会注册同名事件日志来源，一并删除
    Remove-Item "HKLM:\SYSTEM\CurrentControlSet\Services\EventLog\Application\$ServiceName" -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host 'Agent 已卸载' -ForegroundColor Green
}

# 以点号方式加载（测试）时只定义函数
if ($MyInvocation.InvocationName -ne '.') {
    if ($Uninstall) { Uninstall-Agent } else { Install-Agent }
}
