# Casbin Gateway one-step install for Windows.
#
# Usage (run only if you trust this script source):
#   irm https://raw.githubusercontent.com/apache/casbin-gateway/master/scripts/install.ps1 | iex
#
# This downloads the nightly build, which is an automated build of master and
# not an official release. Use it for testing and development only.
#
# Optional environment variables:
#   INSTALL_DIR   where the executable and its data live
#                 (default: $env:LOCALAPPDATA\casbin-gateway)
#   NO_START      set to any value to install without starting Gateway
#   NO_AUTOSTART  set to any value to skip the login-time startup entry
#   NO_SHORTCUT   set to any value to skip the desktop and Start menu shortcuts

$ErrorActionPreference = 'Stop'

$Repo     = 'apache/casbin-gateway'
$Tag      = 'nightly'
$BaseName = 'casbin-gateway-nightly'

$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:LOCALAPPDATA\casbin-gateway" }
$BinDir     = Join-Path $InstallDir 'bin'

# Windows PowerShell 5.1 still negotiates TLS 1.0 by default, which GitHub
# refuses. The progress bar makes Invoke-WebRequest many times slower on a
# download this size.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$ProgressPreference = 'SilentlyContinue'

function Write-Info { param([string]$Message) Write-Host $Message }

# ── pick the archive for this machine ─────────────────────────────────────────
# Only the x86_64 archive is published; Windows on ARM runs it under the
# built-in x64 emulation.
$ArchName = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'x86_64' }
    'ARM64' {
        Write-Host 'No arm64 build is published, installing the x86_64 one to run under emulation'
        'x86_64'
    }
    default { throw "unsupported architecture `"$env:PROCESSOR_ARCHITECTURE`", build from source instead: https://github.com/$Repo" }
}

$Archive = "$BaseName-windows-$ArchName.zip"
$Url     = "https://github.com/$Repo/releases/download/$Tag/$Archive"

$TmpDir = Join-Path $env:TEMP "casbin-gateway-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    # ── download ──────────────────────────────────────────────────────────────
    $ArchivePath = Join-Path $TmpDir $Archive
    Write-Info "Downloading $Url"
    Invoke-WebRequest -Uri $Url -OutFile $ArchivePath -UseBasicParsing

    Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force
    $Unpacked = Join-Path $TmpDir "$BaseName-windows-$ArchName"

    # ── install ───────────────────────────────────────────────────────────────
    Write-Info "Installing to $InstallDir"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

    $DesktopExePath = Join-Path $InstallDir 'casbin-gateway-desktop.exe'
    foreach ($executable in @('casbin-gateway.exe', 'casbin-gateway-desktop.exe')) {
        try {
            Copy-Item -Path (Join-Path $Unpacked $executable) -Destination (Join-Path $InstallDir $executable) -Force
        }
        catch {
            # Windows locks a running executable, so this is what an upgrade over
            # a started Gateway looks like.
            throw "cannot replace $executable in $InstallDir, quit Casbin Gateway from its tray icon and try again ($_)"
        }
    }

    foreach ($legalFile in @('LICENSE', 'NOTICE', 'DISCLAIMER')) {
        Copy-Item -Path (Join-Path $Unpacked $legalFile) -Destination (Join-Path $InstallDir $legalFile) -Force
    }
}
finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}

# ── put a "casbin-gateway" command on PATH ────────────────────────────────────
# Gateway keeps its database, logs and temporary files in the working
# directory, so the command is a wrapper that always starts it in $InstallDir.
# Without it, running "casbin-gateway" from somewhere else would quietly start
# a second, empty installation. The wrapper lives in its own directory because
# cmd resolves .exe before .cmd, so a wrapper next to the executable would
# never be the one that runs.
New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
@"
@echo off
rem Written by the Casbin Gateway installer. Gateway reads and writes .\data,
rem .\logs and .\tmp, so it always has to start in its own directory.
cd /d "%~dp0.." || exit /b 1
"%~dp0..\casbin-gateway.exe" %*
"@ | Set-Content -Path (Join-Path $BinDir 'casbin-gateway.cmd') -Encoding ascii

$UserPath = [System.Environment]::GetEnvironmentVariable('PATH', 'User')
if ($UserPath -notlike "*$BinDir*") {
    [System.Environment]::SetEnvironmentVariable('PATH', "$UserPath;$BinDir", 'User')
    $env:PATH = "$env:PATH;$BinDir"
    Write-Info "Added $BinDir to your PATH, which takes effect in your next terminal"
}

# ── desktop and Start menu shortcuts ──────────────────────────────────────────
# The launcher creates them, so that an install and an archive unpacked by hand
# end up with the same pair. They point at the launcher rather than at the
# server: it is what shows the window, and it starts the server itself if
# nothing is serving yet. Telling it "off" is what keeps a reinstall with
# NO_SHORTCUT from getting them back on the next start.
$ShortcutName = 'Casbin Gateway.lnk'
$ShortcutState = if ($env:NO_SHORTCUT) { 'off' } else { 'on' }

try {
    $shortcut = Start-Process -FilePath $DesktopExePath -ArgumentList 'shortcut', $ShortcutState -WorkingDirectory $InstallDir -Wait -PassThru
    if ($shortcut.ExitCode -ne 0) {
        throw "the launcher exited with $($shortcut.ExitCode)"
    }
}
catch {
    Write-Info "Could not create the shortcuts: $_"
}

# ── start with Windows ────────────────────────────────────────────────────────
# The launcher owns this entry so that the tray's "Start at Login" checkbox and
# the installer are never out of step. An older install put a shortcut in the
# Startup folder instead, which would now start a second copy.
Remove-Item -Path (Join-Path ([System.Environment]::GetFolderPath('Startup')) $ShortcutName) -Force -ErrorAction SilentlyContinue
if (-not $env:NO_AUTOSTART) {
    try {
        $autostart = Start-Process -FilePath $DesktopExePath -ArgumentList 'autostart', 'on' -WorkingDirectory $InstallDir -Wait -PassThru
        if ($autostart.ExitCode -ne 0) {
            throw "the launcher exited with $($autostart.ExitCode)"
        }
        Write-Info 'Casbin Gateway will start with Windows.'
    }
    catch {
        Write-Info "Could not add the startup entry: $_"
    }
}

Write-Info ''
Write-Info "Casbin Gateway is installed in $InstallDir"
Write-Info 'Its database, logs and temporary files stay in that directory.'
Write-Info 'It serves this machine only, and signs you in there as admin without a password.'
Write-Info 'Closing its window leaves it running in the tray; quit it from there.'
Write-Info 'Without the window: "casbin-gateway start", "casbin-gateway stop", "casbin-gateway status".'
Write-Info ''

if ($env:NO_START) {
    Write-Info 'Start it from the "Casbin Gateway" shortcut, or with: casbin-gateway start'
    return
}

# The launcher detaches on its own, so installing does not occupy this terminal.
Start-Process -FilePath $DesktopExePath -WorkingDirectory $InstallDir
