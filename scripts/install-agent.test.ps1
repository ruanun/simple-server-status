# install-agent.ps1 的函数级测试
# 作者: ruan
# 用法: pwsh -File scripts/install-agent.test.ps1（也可用 powershell -File 在 Windows PowerShell 5.1 下运行）
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'install-agent.ps1')

$script:failed = 0
function Check([string]$Name, $Expected, $Actual) {
    if ("$Expected" -eq "$Actual") {
        Write-Host "ok   $Name"
    } else {
        Write-Host "FAIL ${Name}: 期望 [$Expected] 实际 [$Actual]"
        $script:failed = 1
    }
}
function Throws([scriptblock]$Block) {
    try { & $Block | Out-Null; return 'no' } catch { return 'yes' }
}

Check 'latest 地址' 'https://github.com/ruanun/simple-server-status/releases/latest/download/sss-agent_windows_amd64.zip' (Get-DownloadUrl 'latest' 'amd64')
Check '指定版本地址' 'https://github.com/ruanun/simple-server-status/releases/download/v2.0.0/sss-agent_windows_amd64.zip' (Get-DownloadUrl 'v2.0.0' 'amd64')

Check 'YAML 单引号转义' "'it''s'" (ConvertTo-YamlQuoted "it's")
$lines = (Get-AgentConfig 'https://s.example.com' "a'b" '$x y' 'C:\Program Files\sss-agent\logs\agent.log') -split "`n"
Check '配置 dashboard 行' 'True' ($lines -contains "dashboard: 'https://s.example.com'")
Check '配置 id 行' 'True' ($lines -contains "id: 'a''b'")
Check '配置 secret 行不展开' 'True' ($lines -contains "secret: '`$x y'")
Check '配置 log_file 行' 'True' ($lines -contains "log_file: 'C:\Program Files\sss-agent\logs\agent.log'")

# 后台生成的命令：& $f -Dashboard '<地址>' -Id '<ID>' -Secret '<密钥>'
Check '去掉末尾斜杠' 'https://s.example.com' (Assert-InstallArgs 'https://s.example.com/' 'abc' 'x')
Check '缺少 Dashboard 报错' 'yes' (Throws { Assert-InstallArgs '' 'abc' 'x' })
Check '缺少 Id 报错' 'yes' (Throws { Assert-InstallArgs 'https://a' '' 'x' })
Check '缺少 Secret 报错' 'yes' (Throws { Assert-InstallArgs 'https://a' 'abc' '' })
Check '非 http 地址报错' 'yes' (Throws { Assert-InstallArgs 'ftp://a' 'abc' 'x' })

# 两个脚本都必须以 UTF-8 BOM 开头，否则 Windows PowerShell 5.1 会按本地编码解析中文
foreach ($name in 'install-agent.ps1', 'install-agent.test.ps1') {
    $bytes = [IO.File]::ReadAllBytes((Join-Path $PSScriptRoot $name))
    Check "$name 带 UTF-8 BOM" '239,187,191' ($bytes[0..2] -join ',')
}

exit $script:failed
