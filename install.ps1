# Koda installer for Windows
# One-liner: irm https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.ps1 | iex
# With Mouseketool: irm https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.ps1 -OutFile install.ps1; .\install.ps1 -WithMouseketool
# Diagnose: irm https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.ps1 -OutFile install.ps1; .\install.ps1 -Diagnose

param(
    [switch]$WithMouseketool,
    [switch]$Diagnose
)

$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'rsanchez-disney/Koda'
$installDir = if ($env:KODA_INSTALL_DIR) { $env:KODA_INSTALL_DIR } else { "$env:LOCALAPPDATA\koda" }

# Detect architecture
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$binary = "koda-windows-${arch}.exe"

# Diagnostic mode: test connectivity and report issues
if ($Diagnose) {
    Write-Host ''
    Write-Host '   🔍 Koda Install Diagnostics'
    Write-Host '   ==========================='
    Write-Host ''
    Write-Host "   PowerShell: $($PSVersionTable.PSVersion)"
    Write-Host "   OS: $([System.Runtime.InteropServices.RuntimeInformation]::OSDescription)"
    Write-Host "   Arch: $arch"
    Write-Host "   Install dir: $installDir"
    Write-Host ''

    # Test DNS resolution
    Write-Host '   [1/4] DNS resolution for api.github.com...'
    try {
        $dns = [System.Net.Dns]::GetHostAddresses('api.github.com')
        Write-Host "         ✓ Resolved to: $($dns[0].IPAddressToString)"
    } catch {
        Write-Host "         ✗ FAILED: $($_.Exception.Message)"
        Write-Host '         → DNS blocked. Check corporate proxy or VPN.'
    }

    # Test GitHub API
    Write-Host '   [2/4] GitHub API (https://api.github.com/repos/rsanchez-disney/Koda/releases/latest)...'
    try {
        $resp = Invoke-WebRequest -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing -TimeoutSec 10
        Write-Host "         ✓ HTTP $($resp.StatusCode) — API accessible"
        $release = $resp.Content | ConvertFrom-Json
        Write-Host "         ✓ Latest release: $($release.tag_name)"
    } catch {
        $status = $_.Exception.Response.StatusCode.value__
        if ($status -eq 403) {
            Write-Host "         ✗ HTTP 403 — Rate limited. Wait 1 hour or use a GITHUB_TOKEN."
            # Check rate limit
            try {
                $rl = Invoke-RestMethod "https://api.github.com/rate_limit"
                Write-Host "         → Remaining: $($rl.rate.remaining)/$($rl.rate.limit)"
                $reset = [DateTimeOffset]::FromUnixTimeSeconds($rl.rate.reset).LocalDateTime
                Write-Host "         → Resets at: $reset"
            } catch {}
        } elseif ($status -eq 404) {
            Write-Host "         ✗ HTTP 404 — Repository not found or private."
        } else {
            Write-Host "         ✗ FAILED: $($_.Exception.Message)"
            Write-Host "         → Likely a proxy, firewall, or TLS interception issue."
        }
    }

    # Test direct download URL
    Write-Host '   [3/4] Direct download redirect (https://github.com/rsanchez-disney/Koda/releases/latest/download/koda-windows-amd64.exe)...'
    try {
        $headResp = Invoke-WebRequest -Uri "https://github.com/$repo/releases/latest/download/$binary" -Method Head -UseBasicParsing -TimeoutSec 10 -MaximumRedirection 0 -ErrorAction SilentlyContinue
        Write-Host "         ✓ Redirect available (HTTP $($headResp.StatusCode))"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 302) {
            Write-Host "         ✓ Redirect available (HTTP 302)"
        } else {
            Write-Host "         ✗ FAILED: $($_.Exception.Message)"
        }
    }

    # Test proxy settings
    Write-Host '   [4/4] Proxy configuration...'
    $proxy = [System.Net.WebRequest]::GetSystemWebProxy()
    $testUri = [System.Uri]::new("https://api.github.com")
    $proxyUri = $proxy.GetProxy($testUri)
    if ($proxyUri.ToString() -ne $testUri.ToString()) {
        Write-Host "         ⚠ Proxy detected: $proxyUri"
        Write-Host "         → Proxy may be blocking or modifying GitHub requests."
    } else {
        Write-Host "         ✓ No proxy configured for github.com"
    }

    Write-Host ''
    Write-Host '   Done. Share this output when reporting install issues.'
    Write-Host ''
    exit 0
}

Write-Host ''
Write-Host '   Installing Koda...'
Write-Host "   OS: windows  Arch: $arch"
Write-Host ''

# Find latest release — try API first, fall back to direct download
$tag = $null
$url = $null

try {
    $release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest" -TimeoutSec 15
    $tag = $release.tag_name
    $url = "https://github.com/$repo/releases/download/$tag/$binary"
} catch {
    $errMsg = $_.Exception.Message
    $status = $_.Exception.Response.StatusCode.value__

    Write-Host "   ⚠ GitHub API unavailable (HTTP $status): $errMsg"
    Write-Host '   → Falling back to direct download...'
    Write-Host ''

    # Fallback: use /releases/latest/download/ which redirects without needing the API
    $url = "https://github.com/$repo/releases/latest/download/$binary"
    $tag = 'latest'
}

Write-Host "   Version: $tag"
Write-Host ''

# Download
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$dest = Join-Path $installDir 'koda.exe'

try {
    Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing -TimeoutSec 120
} catch {
    Write-Host "   ✗ Download failed: $($_.Exception.Message)"
    Write-Host ''
    Write-Host '   Troubleshooting:'
    Write-Host '     1. Run with -Diagnose flag for detailed connectivity test:'
    Write-Host '        irm https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.ps1 -OutFile install.ps1; .\install.ps1 -Diagnose'
    Write-Host ''
    Write-Host '     2. Try direct download in browser:'
    Write-Host "        https://github.com/$repo/releases/latest/download/$binary"
    Write-Host ''
    Write-Host '     3. If behind a proxy, try:'
    Write-Host '        $env:HTTPS_PROXY = "http://your-proxy:port"'
    Write-Host ''
    exit 1
}

if (Test-Path $dest) {
    Write-Host "   ✓ Installed: $dest"
    & $dest version
    Write-Host ''
    if ($env:PATH -notlike "*$installDir*") {
        Write-Host '   Add to PATH (run once, then restart terminal):'
        Write-Host "     [Environment]::SetEnvironmentVariable('PATH', `"$installDir;`$env:PATH`", 'User')"
        Write-Host ''
    }
} else {
    Write-Host '   ✗ Installation failed.'
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
        # Use curl.exe to avoid corporate proxy TLS issues with PowerShell irm
        curl.exe -fsSL https://cli.kiro.dev/install.ps1 | powershell -NoProfile -Command -
        Write-Host '   kiro-cli installed.'
    } catch {
        Write-Host '   kiro-cli auto-install failed. Install manually:'
        Write-Host '     curl.exe -fsSL https://cli.kiro.dev/install.ps1 | powershell -NoProfile -Command -'
        Write-Host '     Or: winget install Kiro.CLI'
    }
}

# Optional: install Mouseketool
if ($WithMouseketool) {
    Write-Host ''
    Write-Host '   📦 Installing Mouseketool...'
    & $dest apps install mouseketool
}
