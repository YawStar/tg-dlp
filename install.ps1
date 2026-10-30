<#
.SYNOPSIS
    Install tg-dlp on Windows.
.DESCRIPTION
    Downloads the latest release binary from GitHub and installs to
    $env:LOCALAPPDATA\tg-dlp, then adds it to the user PATH.
.PARAMETER Version
    Release version to install (default: latest).
.PARAMETER InstallDir
    Custom install directory (default: $env:LOCALAPPDATA\tg-dlp).
.EXAMPLE
    iwr -useb https://raw.githubusercontent.com/YawStar/tg-dlp/main/scripts/install.ps1 | iex
#>

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\tg-dlp"
)

$ErrorActionPreference = "Stop"

# --- Config ---
$Repo = "YawStar/tg-dlp"
$Binary = "tg-dlp"

# --- Detect arch ---
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default {
        Write-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
        exit 1
    }
}

# --- Resolve version ---
if ($Version -eq "latest") {
    Write-Host "🔍 Fetching latest release..." -ForegroundColor Cyan
    $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $Version = $release.tag_name
}

$VersionClean = $Version.TrimStart("v")
$Archive = "${Binary}_${VersionClean}_windows_${arch}.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$Archive"

Write-Host "📦 Downloading $Archive..." -ForegroundColor Cyan

# --- Create temp dir ---
$tmpDir = Join-Path $env:TEMP "tg-dlp-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    $tmpZip = Join-Path $tmpDir $Archive
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $tmpZip -UseBasicParsing

    Write-Host "📂 Extracting..." -ForegroundColor Cyan
    Expand-Archive -Path $tmpZip -DestinationPath $tmpDir -Force

    # Find binary
    $exe = Get-ChildItem -Path $tmpDir -Filter "$Binary.exe" -Recurse | Select-Object -First 1
    if (-not $exe) {
        Write-Error "Could not find $Binary.exe in the archive."
        exit 1
    }

    # --- Install ---
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $dest = Join-Path $InstallDir "$Binary.exe"
    Copy-Item -Path $exe.FullName -Destination $dest -Force

    Write-Host "✅ Installed to $dest" -ForegroundColor Green

    # --- Add to user PATH if missing ---
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        Write-Host "🔧 Adding $InstallDir to user PATH..." -ForegroundColor Cyan
        [Environment]::SetEnvironmentVariable(
            "Path",
            "$userPath;$InstallDir",
            "User"
        )
        Write-Host "   Restart your terminal for PATH changes to take effect." -ForegroundColor Yellow
    }

    # --- Verify ---
    & $dest version
} finally {
    Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}