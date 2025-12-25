$ErrorActionPreference = "Stop"

# 列出需执行单测的包，跳过包含空格的 personal blockchain 目录
$packages = @(
    "./core",
    "./crypto",
    "./storage",
    "./network",
    "./cmd/node",
    "./config",
    "./test"
)

Write-Host "Running go test on packages:"
Write-Host ($packages -join "`n")

go test $packages

if ($LASTEXITCODE -eq 0) {
    Write-Host "All selected tests passed." -ForegroundColor Green
}

