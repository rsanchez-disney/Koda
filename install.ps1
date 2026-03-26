
# Koda installer for Windows
# One-liner: irm https://github.disney.com/raw/SANCR225/Koda/main/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$repo = 'SANCR225/Koda'
$ghHost = 'github.disney.com'
$installDir = if ($env:KODA_INSTALL_DIR) { $env:KODA_INSTALL_DIR } else { "$env:LOCALAPPDATA\koda" }
$binary = 'koda-windows-amd64.exe'

Write-Host ''
Write-Host '   Installing Koda...'
Write-Host "   OS: windows  Arch: amd64"
Write-Host ''

# Find latest release
try {
    $release = Invoke-RestMethod "https://$ghHost/api/v3/repos/$repo/releases/latest"
    $tag = $release.tag_name
} catch {
    Write-Host '   Could not determine latest release.'
    exit 1
}

$url = "https://$ghHost/$repo/releases/download/$tag/$binary"
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
    # Check PATH
    if ($env:PATH -notlike "*$installDir*") {
        Write-Host '   Add to PATH:'
        Write-Host "     [Environment]::SetEnvironmentVariable('PATH', \"$installDir;\$env:PATH\", 'User')"
        Write-Host ''
    }
} else {
    Write-Host '   Installation failed.'
    exit 1
}
