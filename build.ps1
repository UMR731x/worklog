$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

New-Item -ItemType Directory -Force dist/windows-amd64, dist/linux-amd64 | Out-Null
$previousCGO = $env:CGO_ENABLED
$previousOS = $env:GOOS
$previousArch = $env:GOARCH
$buildVersion = if ($env:VERSION) { $env:VERSION } else { "v0.3.0-dev" }

try {
    $env:CGO_ENABLED = "0"
    $env:GOARCH = "amd64"
    $env:GOOS = "windows"
    go build -buildvcs=false -trimpath "-ldflags=-s -w -H windowsgui -X worklog/internal/update.Version=$buildVersion" -o dist/windows-amd64/worklog.exe .
    if ($LASTEXITCODE -ne 0) { throw "Windows 构建失败" }

    $env:GOOS = "linux"
    go build -buildvcs=false -trimpath "-ldflags=-s -w -X worklog/internal/update.Version=$buildVersion" -o dist/linux-amd64/worklog .
    if ($LASTEXITCODE -ne 0) { throw "Linux 构建失败" }
    Copy-Item THIRD_PARTY_NOTICES.md dist/windows-amd64/
    Copy-Item THIRD_PARTY_NOTICES.md dist/linux-amd64/
} finally {
    $env:CGO_ENABLED = $previousCGO
    $env:GOOS = $previousOS
    $env:GOARCH = $previousArch
}

Write-Host "构建完成：dist/windows-amd64/worklog.exe 和 dist/linux-amd64/worklog"
