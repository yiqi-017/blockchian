<#
停止通过 start_all.ps1 启动的进程，依据 test/logs/pids.txt
用法：
  pwsh -File test/stop_all.ps1
#>

$repoRoot = (Resolve-Path "$PSScriptRoot/..").Path
$pidFile = Join-Path $repoRoot "test/logs/pids.txt"

if (-not (Test-Path $pidFile)) {
    Write-Host "未找到 $pidFile ，无需停止。" -ForegroundColor Yellow
    exit 0
}

$lines = Get-Content $pidFile
foreach ($line in $lines) {
    if ($line -match "^\s*(\S+)\s+(\d+)\s*$") {
        $name = $matches[1]
        $procId = [int]$matches[2]
        try {
            Write-Host ("Stopping {0} (PID {1})" -f $name, $procId)
            Stop-Process -Id $procId -Force -ErrorAction Stop
        } catch {
            Write-Host ("  无法停止 {0}: {1}" -f $name, $_.Exception.Message) -ForegroundColor Yellow
        }
    }
}

Remove-Item $pidFile -Force -ErrorAction SilentlyContinue

