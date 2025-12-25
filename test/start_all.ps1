<#
一键启动节点 + dashboard + 前端（使用 Start-Process，写日志，记录 PID）
运行：pwsh -File test/start_all.ps1
停止：pwsh -File test/stop_all.ps1
#>

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path "$PSScriptRoot/..").Path
Set-Location $repoRoot

$ports = @{ n1=":8080"; n2=":8081"; n3=":8082" }
$dashboardPort = ":4000"
$webPort = 3000
$logDir = Join-Path $repoRoot "test/logs"
New-Item -ItemType Directory -Path $logDir -Force | Out-Null
$pidFile = Join-Path $logDir "pids.txt"
if (Test-Path $pidFile) { Remove-Item $pidFile -Force }

function Has-Blocks {
    param([string]$Node)
    $path = Join-Path $repoRoot "data/$Node/blocks"
    if (-not (Test-Path $path)) { return $false }
    $files = Get-ChildItem $path -Filter "*.json" -ErrorAction SilentlyContinue
    return $files.Count -gt 0
}

function Start-Proc {
    param(
        [string]$Name,
        [string]$Command,
        [string]$WorkingDir = $repoRoot
    )
    $log = Join-Path $logDir "$Name.log"
    # 使用 cmd 重定向到单个日志文件
    $arg = "/c $Command > `"$log`" 2>&1"
    $proc = Start-Process -FilePath "cmd.exe" `
        -ArgumentList $arg `
        -WorkingDirectory $WorkingDir `
        -NoNewWindow `
        -PassThru
    Add-Content -Path $pidFile -Value "$Name $($proc.Id)"
    Write-Host ("  [{0}] PID={1} log={2}" -f $Name, $proc.Id, $log) -ForegroundColor DarkGray
}

# 1) init 创世（仅当无数据）
$hasData = Has-Blocks -Node "n1"
if ($hasData) {
    Write-Host "1) 检测到已有链数据，跳过 init" -ForegroundColor Yellow
} else {
    Write-Host "1) 初始化创世块 (n1)" -ForegroundColor Cyan
    & go run ./cmd/node -mode init -node n1
}

# 2) 启动节点
Write-Host "2) 启动 3 个节点 serve (后台进程)" -ForegroundColor Cyan
Start-Proc -Name "n1" -Command "go run ./cmd/node -mode serve -node n1 -addr $($ports.n1) -peers http://127.0.0.1:8081,http://127.0.0.1:8082"
Start-Proc -Name "n2" -Command "go run ./cmd/node -mode serve -node n2 -addr $($ports.n2) -peers http://127.0.0.1:8080,http://127.0.0.1:8082"
Start-Proc -Name "n3" -Command "go run ./cmd/node -mode serve -node n3 -addr $($ports.n3) -peers http://127.0.0.1:8080,http://127.0.0.1:8081"

Start-Sleep -Seconds 2

# 3) 启动 dashboard
Write-Host "3) 启动 dashboard 代理 (后台进程)" -ForegroundColor Cyan
Start-Proc -Name "dashboard" -Command "go run ./cmd/dashboard -addr $dashboardPort -peers http://127.0.0.1:8080,http://127.0.0.1:8081,http://127.0.0.1:8082"

# 4) 启动前端
Write-Host ("4) 启动静态前端 (npx serve) -> http://localhost:{0}" -f $webPort) -ForegroundColor Cyan
Start-Proc -Name "web" -Command "npx serve web -l $webPort"

Write-Host "`n全部启动完毕：" -ForegroundColor Green
Write-Host "  Dashboard: http://127.0.0.1$dashboardPort" -ForegroundColor Green
Write-Host ("  前端页面: http://localhost:{0} (API 基址保持 http://127.0.0.1{1})" -f $webPort, $dashboardPort) -ForegroundColor Green
Write-Host ("  PID 列表保存在 {0}，日志在 test/logs/*.log" -f $pidFile) -ForegroundColor Yellow
Write-Host "停止脚本：pwsh -File test/stop_all.ps1" -ForegroundColor Yellow

