# Builds the app, embeds it in the custom installer, and writes
# installer\build\bin\PC-Optimizer-Setup.exe
#
# Google sign-in: put your OAuth "Desktop app" credentials in .env.local
# (git-ignored), or set the same names as environment variables:
#   PCOPT_GOOGLE_CLIENT_ID=...
#   PCOPT_GOOGLE_CLIENT_SECRET=...
# They are compiled into the app so users only click "Sign in with Google".
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if (Test-Path .env.local) {
  Get-Content .env.local | ForEach-Object {
    if ($_ -match '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$' -and -not $_.TrimStart().StartsWith('#')) {
      [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2].Trim('"'), "Process")
    }
  }
}

$version = (Get-Content wails.json | ConvertFrom-Json).info.productVersion
$id = $env:PCOPT_GOOGLE_CLIENT_ID
$secret = $env:PCOPT_GOOGLE_CLIENT_SECRET

$ldflags = ""
if ($id) {
  $ldflags = "-X pcoptimizer/internal/auth.GoogleClientID=$id"
  if ($secret) { $ldflags += " -X pcoptimizer/internal/auth.GoogleClientSecret=$secret" }
} else {
  Write-Warning "PCOPT_GOOGLE_CLIENT_ID is not set. This build has NO sign-in and opens straight into the app."
}

Write-Host "1/3 Building the app..."
if ($ldflags) { wails build -clean -ldflags $ldflags } else { wails build -clean }
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
