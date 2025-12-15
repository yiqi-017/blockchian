<#
运行示例：
  pwsh -File test/demo.ps1

需要：已安装 Go，当前目录为仓库根或任意位置均可。
脚本流程：创世 -> 启动 3 节点 serve -> 发送交易 -> 挖块 -> 查询状态 -> 清理后台任务。
#>

$repoRoot = (Resolve-Path "$PSScriptRoot/..").Path
Set-Location $repoRoot

function Start-Node {
    param(
        [string]$Name,
        [string]$Addr,
        [string]$Peers
    )
    Start-Job -Name $Name -ScriptBlock {
        param($root, $name, $addr, $peers)
        Set-Location $root
        go run ./cmd/node -mode serve -node $name -addr $addr -peers $peers
    } -ArgumentList $repoRoot, $Name, $Addr, $Peers | Out-Null
}

function Stop-Nodes {
    param([string[]]$Names)
    foreach ($n in $Names) {
        if (Get-Job -Name $n -ErrorAction SilentlyContinue) {
            Stop-Job -Name $n -ErrorAction SilentlyContinue
            Remove-Job -Name $n -ErrorAction SilentlyContinue
        }
    }
}

$nodes = @("n1","n2","n3")
try {
    Write-Host "1) 初始化创世块 (n1)" -ForegroundColor Cyan
    go run ./cmd/node -mode init -node n1

    Write-Host "2) 启动 3 个节点 serve (后台任务)" -ForegroundColor Cyan
    Start-Node -Name n1 -Addr ":8080" -Peers "http://127.0.0.1:8081,http://127.0.0.1:8082"
    Start-Node -Name n2 -Addr ":8081" -Peers "http://127.0.0.1:8080,http://127.0.0.1:8082"
    Start-Node -Name n3 -Addr ":8082" -Peers "http://127.0.0.1:8080,http://127.0.0.1:8081"
    Start-Sleep -Seconds 3

    Write-Host "3) 发送交易 (n1 -> alice)" -ForegroundColor Cyan
    go run ./cmd/node -mode tx -node n1 -to alice -value 12

    Write-Host "4) 挖块 (n1)" -ForegroundColor Cyan
    go run ./cmd/node -mode mine -node n1 -miner miner1 -difficulty 8

    Write-Host "5) 等待同步..." -ForegroundColor Cyan
    Start-Sleep -Seconds 5

    Write-Host "6) 查询各节点状态" -ForegroundColor Cyan
    $urls = @(
        "http://127.0.0.1:8080/status",
        "http://127.0.0.1:8081/status",
        "http://127.0.0.1:8082/status"
    )
    foreach ($u in $urls) {
        try {
            $resp = Invoke-RestMethod -Uri $u -Method Get
            Write-Host "$u -> height=$($resp.height) node=$($resp.node_id)"
        } catch {
            Write-Warning "$u -> $($_.Exception.Message)"
        }
    }

    Write-Host "区块文件位于 data/n*/blocks/*.json" -ForegroundColor Green
}
finally {
    Write-Host "清理后台节点进程..." -ForegroundColor Yellow
    Stop-Nodes -Names $nodes
}

