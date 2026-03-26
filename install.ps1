# Koda installer for Windows
# One-liner: irm https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.ps1 | iex

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'rsanchez-disney/Koda'
$installDir = if ($env:KODA_INSTALL_DIR) { $env:KODA_INSTALL_DIR } else { "$env:LOCALAPPDATA\koda" }

# Windows ARM64 runs amd64 via emulation
$arch = 'amd64'
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

# Get direct download URL from release assets
$asset = $release.assets | Where-Object { $_.name -eq $binary }
if (-not $asset) {
    Write-Host "   Binary $binary not found in release $tag"
    exit 1
}
$url = $asset.browser_download_url

Write-Host "   Version: $tag"
Write-Host ''

# Download
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$dest = Join-Path $installDir 'koda.exe'
Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing

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
