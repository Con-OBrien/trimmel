# Builds the app, embeds it in the custom installer, and writes
# installer\build\bin\PC-Optimizer-Setup.exe
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$version = (Get-Content wails.json | ConvertFrom-Json).info.productVersion

Write-Host "1/3 Building the app..."
wails build -clean
if ($LASTEXITCODE) { throw "app build failed" }
Copy-Item build\bin\pcoptimizer.exe installer\payload\pcoptimizer.exe -Force

Write-Host "2/3 Building the installer (v$version)..."
Push-Location installer
try {
  go mod tidy
  wails build -clean -ldflags "-X main.version=$version"
  if ($LASTEXITCODE) { throw "installer build failed" }
} finally { Pop-Location }

Write-Host "3/3 Done: installer\build\bin\PC-Optimizer-Setup.exe"
