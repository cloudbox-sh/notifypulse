# Notifypulse CLI installer for Windows (PowerShell 5.1+ / PowerShell 7+).
#
# Usage:
#   irm https://get.cloudbox.sh/notifypulse.ps1 | iex
#   $env:INSTALL_VERSION = 'v0.2.0'; irm https://get.cloudbox.sh/notifypulse.ps1 | iex
#
# Environment:
#   INSTALL_VERSION  pin a specific tag (e.g. v0.2.0). Default: latest release.
#   INSTALL_DIR      install target. Default: %LOCALAPPDATA%\Programs\Notifypulse.
#
# Downloads the matching GoReleaser zip, verifies SHA-256 against checksums.txt,
# extracts the binary to INSTALL_DIR, and adds it to the user PATH.

$ErrorActionPreference = 'Stop'

$Owner = 'cloudbox-sh'
$Repo  = 'notifypulse'
$Bin   = 'notifypulse.exe'

function Info($msg) { Write-Host "==> $msg" -ForegroundColor Magenta }
function Warn($msg) { Write-Host "!! $msg" -ForegroundColor Yellow }
function Fail($msg) { Write-Host "xx $msg" -ForegroundColor Red; exit 1 }

# Detect architecture.
$arch = switch -Wildcard ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { 'x86_64' }
  'ARM64' { Fail 'windows/arm64 is not yet shipped; use npm or `go install` instead.' }
  default { Fail "unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)" }
}

# Resolve version.
$version = $env:INSTALL_VERSION
if (-not $version) {
  Info 'resolving latest release tag'
  try {
    $rel = Invoke-RestMethod -UseBasicParsing -Uri "https://api.github.com/repos/$Owner/$Repo/releases/latest"
    $version = $rel.tag_name
  } catch {
    Fail "could not determine latest release tag: $($_.Exception.Message)"
  }
}
$versionNoV = $version.TrimStart('v')

$archive   = "notifypulse_${versionNoV}_windows_${arch}.zip"
$checksums = 'checksums.txt'
$baseUrl   = "https://github.com/$Owner/$Repo/releases/download/$version"

# Pick install dir.
$installDir = $env:INSTALL_DIR
if (-not $installDir) {
  $installDir = Join-Path $env:LOCALAPPDATA 'Programs\Notifypulse'
}
if (-not (Test-Path $installDir)) {
  New-Item -ItemType Directory -Path $installDir -Force | Out-Null
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  $archivePath   = Join-Path $tmp $archive
  $checksumsPath = Join-Path $tmp $checksums

  Info "downloading $archive ($version)"
  Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$archive"   -OutFile $archivePath

  Info "downloading $checksums"
  Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$checksums" -OutFile $checksumsPath

  Info 'verifying SHA-256'
  $expectedLine = (Get-Content $checksumsPath) | Where-Object { $_ -match "\s$([regex]::Escape($archive))$" } | Select-Object -First 1
  if (-not $expectedLine) { Fail 'archive not listed in checksums.txt' }
  $expected = ($expectedLine -split '\s+')[0].ToLowerInvariant()
  $actual   = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($expected -ne $actual) { Fail "checksum mismatch: expected $expected, got $actual" }

  Info 'extracting'
  Expand-Archive -Path $archivePath -DestinationPath $tmp -Force

  $srcBin = Join-Path $tmp $Bin
  if (-not (Test-Path $srcBin)) { Fail "binary $Bin not found in archive" }

  Info "installing to $installDir"
  Copy-Item -Path $srcBin -Destination (Join-Path $installDir $Bin) -Force

  # Add to user PATH if missing.
  $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (-not $userPath) { $userPath = '' }
  $paths = $userPath -split ';' | Where-Object { $_ -ne '' }
  if ($paths -notcontains $installDir) {
    Info "adding $installDir to user PATH"
    [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $installDir), 'User')
    Warn 'open a new shell for PATH changes to take effect.'
  }

  Info "installed notifypulse $version to $installDir\$Bin"
  & (Join-Path $installDir $Bin) version
}
finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
