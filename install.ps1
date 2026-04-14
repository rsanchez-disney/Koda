# Koda installer for Windows
# One-liner: irm https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.ps1 | iex

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'rsanchez-disney/Koda'
$installDir = if ($env:KODA_INSTALL_DIR) { $env:KODA_INSTALL_DIR } else { "$env:LOCALAPPDATA\koda" }

# Detect architecture
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$binary = "koda-windows-${arch}.exe"

Write-Host ''
Write-Host '   Installing Koda...'
Write-Host "   OS: windows  Arch: $arch"
Write-Host ''

# Find latest release
try {
    $release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
    $tag = $release.tag_name
} catch {
    Write-Host '   Could not determine latest release.'
    exit 1
}

$url = "https://github.com/$repo/releases/download/$tag/$binary"
Write-Host "   Version: $tag"
Write-Host "   URL: $url"
Write-Host ''

# Download
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$dest = Join-Path $installDir 'koda.exe'
Invoke-WebRequest -Uri $url -OutFile $dest

if (Test-Path $dest) {
    Write-Host "   Installed: $dest"
    & $dest version
    Write-Host ''
    if ($env:PATH -notlike "*$installDir*") {
        Write-Host '   Add to PATH:'
        Write-Host "     [Environment]::SetEnvironmentVariable('PATH', `"$installDir;`$env:PATH`", 'User')"
        Write-Host ''
    }
} else {
    Write-Host '   Installation failed.'
    exit 1
}

# Check for kiro-cli
Write-Host ''
if (Get-Command kiro-cli -ErrorAction SilentlyContinue) {
    $kiroVer = & kiro-cli --version 2>&1
    Write-Host "   kiro-cli: $kiroVer"
} else {
    Write-Host '   kiro-cli not found. Installing from https://kiro.dev/cli/...'
    try {
        irm https://cli.kiro.dev/install.ps1 | iex
        Write-Host '   kiro-cli installed.'
    } catch {
        Write-Host '   kiro-cli auto-install failed. Install manually:'
        Write-Host '     irm https://cli.kiro.dev/install.ps1 | iex'
        Write-Host '     Or: winget install Kiro.CLI'
    }
}
