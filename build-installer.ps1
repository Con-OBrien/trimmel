# Builds the app, embeds it in the custom installer, and writes
# installer\build\bin\Trimmel-Setup.exe
#
#   .\build-installer.ps1                 build only
#   .\build-installer.ps1 -Publish        also copy the installer and a portable zip to docs\downloads
#
# Sign-in: Google sign-in is optional. To include it, put your OAuth "Desktop app"
# credentials in .env.local (git-ignored), or set the same names as environment variables:
#   TRIMMEL_GOOGLE_CLIENT_ID=...
#   TRIMMEL_GOOGLE_CLIENT_SECRET=...
# They are compiled into the app. Without them the build has no sign-in and opens straight
# into the app. -Publish refuses to publish such a build unless you pass -NoSignIn, so a
# missing .env.local can never silently ship a build without sign-in.
#
# Code signing: optional. When a certificate is available, trimmel.exe and the installer are
# signed with signtool and a timestamp. Set these in .env.local or the environment:
#   TRIMMEL_SIGN_THUMBPRINT=...   SHA-1 thumbprint of the code signing certificate in the Windows
#                                 certificate store (a hardware token or Certum SimplySign shows
#                                 its certificate there once the card is connected)
#   TRIMMEL_SIGN_TIMESTAMP=...    timestamp server URL (default http://time.certum.pl)
# Without a thumbprint the build is unsigned and prints a warning. -Publish also warns, so an
# unsigned release is never silent. Pass -RequireSigned to make -Publish fail instead.
param([switch]$Publish, [switch]$NoSignIn, [switch]$RequireSigned)
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
$id = if ($env:TRIMMEL_GOOGLE_CLIENT_ID) { $env:TRIMMEL_GOOGLE_CLIENT_ID } else { $env:PCOPT_GOOGLE_CLIENT_ID }  # PCOPT_ = pre-0.2.0 name
$secret = if ($env:TRIMMEL_GOOGLE_CLIENT_SECRET) { $env:TRIMMEL_GOOGLE_CLIENT_SECRET } else { $env:PCOPT_GOOGLE_CLIENT_SECRET }

$thumb = $env:TRIMMEL_SIGN_THUMBPRINT
$tsa = if ($env:TRIMMEL_SIGN_TIMESTAMP) { $env:TRIMMEL_SIGN_TIMESTAMP } else { "http://time.certum.pl" }
$signtool = $null
if ($thumb) {
  $signtool = (Get-Command signtool.exe -ErrorAction SilentlyContinue).Source
  if (-not $signtool) {
    $signtool = Get-ChildItem (Join-Path ${env:ProgramFiles(x86)} "Windows Kits/10/bin") -Recurse -Filter signtool.exe -ErrorAction SilentlyContinue |
      Where-Object { $_.FullName -like '*x64*' } | Sort-Object FullName -Descending | Select-Object -First 1 -ExpandProperty FullName
  }
  if (-not $signtool) { throw "TRIMMEL_SIGN_THUMBPRINT is set but signtool.exe was not found. Install the Windows SDK (Signing Tools)." }
} elseif ($Publish -and $RequireSigned) {
  throw "TRIMMEL_SIGN_THUMBPRINT is not set, so this build would be unsigned. Remove -RequireSigned to publish it anyway."
} else {
  Write-Warning "No code signing certificate: the exe and installer will be UNSIGNED (Windows SmartScreen will warn)."
}

# Signs a file when a certificate is configured, then checks the signature.
function Sign-File([string]$path) {
  if (-not $signtool) { return }
  & $signtool sign /sha1 $thumb /fd sha256 /tr $tsa /td sha256 /d "Trimmel" $path
  if ($LASTEXITCODE) { throw "signing failed: $path" }
  & $signtool verify /pa $path | Out-Null
  if ($LASTEXITCODE) { throw "signature check failed: $path" }
}

$ldflags = "-X main.version=$version"
if ($id) {
  $ldflags += " -X trimmel/internal/auth.GoogleClientID=$id"
  if ($secret) { $ldflags += " -X trimmel/internal/auth.GoogleClientSecret=$secret" }
} else {
  if ($Publish -and -not $NoSignIn) { throw "TRIMMEL_GOOGLE_CLIENT_ID is not set (add it to .env.local), so this build has no sign-in. Pass -NoSignIn to publish it anyway." }
  Write-Warning "No Google client ID: this build has NO sign-in and opens straight into the app."
}

Write-Host "1/3 Building the app (v$version)..."
wails build -clean -trimpath -webview2 embed -ldflags $ldflags
if ($LASTEXITCODE) { throw "app build failed" }
Sign-File (Resolve-Path build\bin\trimmel.exe).Path
Copy-Item build\bin\trimmel.exe installer\payload\trimmel.exe -Force

Write-Host "2/3 Building the installer..."
Push-Location installer
try {
  go mod tidy
  wails build -clean -trimpath -webview2 embed -ldflags "-X main.version=$version"
  if ($LASTEXITCODE) { throw "installer build failed" }
} finally { Pop-Location }

Sign-File (Resolve-Path installer\build\bin\Trimmel-Setup.exe).Path

Write-Host "3/3 Done: installer\build\bin\Trimmel-Setup.exe"

# -Publish copies the installer into docs\downloads so Vercel serves it, and
# writes version.json for the download section of the site and the in-app update check.
if ($Publish) {
  New-Item -ItemType Directory -Force docs\downloads | Out-Null
  Copy-Item installer\build\bin\Trimmel-Setup.exe docs\downloads\Trimmel-Setup.exe -Force
  $f = Get-Item docs\downloads\Trimmel-Setup.exe
  # Portable build: just the app exe in a zip, no installer behaviour.
  $zip = "docs\downloads\Trimmel-Portable.zip"
  Remove-Item $zip -ErrorAction SilentlyContinue
  Compress-Archive -Path build\bin\trimmel.exe -DestinationPath $zip
  $z = Get-Item $zip
  @{ version = $version; bytes = $f.Length; date = (Get-Date -Format "d MMM yyyy");
     sha256 = (Get-FileHash $f.FullName -Algorithm SHA256).Hash.ToLower();
     zipBytes = $z.Length; zipSha256 = (Get-FileHash $z.FullName -Algorithm SHA256).Hash.ToLower() } |
    ConvertTo-Json | Set-Content docs\downloads\version.json -Encoding utf8
  Write-Host "Published to docs\downloads. Commit and push to release."
}
