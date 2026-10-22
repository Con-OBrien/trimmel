# Builds the app, embeds it in the custom installer, and writes
# installer\build\bin\PC-Optimizer-Setup.exe
#
# Google sign-in: put your OAuth "Desktop app" credentials in .env.local
# (git-ignored), or set the same names as environment variables:
#   PCOPT_GOOGLE_CLIENT_ID=...
#   PCOPT_GOOGLE_CLIENT_SECRET=...
# They are compiled into the app so users only click "Sign in with Google".
param([switch]$Publish)
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

$ldflags = "-X main.version=$version"
if ($id) {
  $ldflags += " -X pcoptimizer/internal/auth.GoogleClientID=$id"
  if ($secret) { $ldflags += " -X pcoptimizer/internal/auth.GoogleClientSecret=$secret" }
} else {
  if ($Publish) { throw "PCOPT_GOOGLE_CLIENT_ID is not set (add it to .env.local). Refusing to publish a build with no sign-in." }
  Write-Warning "PCOPT_GOOGLE_CLIENT_ID is not set. This build has NO sign-in and opens straight into the app."
}

Write-Host "1/3 Building the app..."
wails build -clean -trimpath -webview2 embed -ldflags $ldflags
if ($LASTEXITCODE) { throw "app build failed" }
Copy-Item build\bin\pcoptimizer.exe installer\payload\pcoptimizer.exe -Force

Write-Host "2/3 Building the installer (v$version)..."
Push-Location installer
try {
  go mod tidy
  wails build -clean -trimpath -webview2 embed -ldflags "-X main.version=$version"
  if ($LASTEXITCODE) { throw "installer build failed" }
} finally { Pop-Location }

Write-Host "3/3 Done: installer\build\bin\PC-Optimizer-Setup.exe"

# -Publish copies the installer into docs\downloads so Vercel serves it, and
# writes version.json for the download section of the site.
if ($Publish) {
  New-Item -ItemType Directory -Force docs\downloads | Out-Null
  Copy-Item installer\build\bin\PC-Optimizer-Setup.exe docs\downloads\PC-Optimizer-Setup.exe -Force
  $f = Get-Item docs\downloads\PC-Optimizer-Setup.exe
  # Portable build: just the app exe in a zip, no installer behaviour.
  $zip = "docs\downloads\PC-Optimizer-Portable.zip"
  Remove-Item $zip -ErrorAction SilentlyContinue
  Compress-Archive -Path build\bin\pcoptimizer.exe -DestinationPath $zip
  $z = Get-Item $zip
  @{ version = $version; bytes = $f.Length; date = (Get-Date -Format "d MMM yyyy");
     sha256 = (Get-FileHash $f.FullName -Algorithm SHA256).Hash.ToLower();
     zipBytes = $z.Length; zipSha256 = (Get-FileHash $z.FullName -Algorithm SHA256).Hash.ToLower() } |
    ConvertTo-Json | Set-Content docs\downloads\version.json -Encoding utf8
  Write-Host "Published to docs\downloads. Commit and push to release."
}
