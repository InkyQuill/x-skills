$ErrorActionPreference = "Stop"

$BinName = "x-skills"
$InstallDir = if ($env:X_SKILLS_INSTALL_DIR) { $env:X_SKILLS_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir

function Write-Step {
    param([string]$Message)
    Write-Host "x-skills install: $Message"
}

function Install-XsShortcut {
    if (Get-Command xs -ErrorAction SilentlyContinue) {
        Write-Step "xs already exists; leaving it unchanged"
        return
    }

    $shortcut = Join-Path $InstallDir "xs.cmd"
    if (Test-Path $shortcut) {
        Write-Step "$shortcut already exists; leaving it unchanged"
        return
    }

    Write-Step "Creating xs shortcut at $shortcut"
    "@echo off`r`n`"%~dp0x-skills.exe`" %*`r`n" | Set-Content -Encoding ASCII -NoNewline $shortcut
}

Write-Step "Starting development installer"
Write-Step "Using install directory $InstallDir"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
$builtExe = Join-Path $tmp "$BinName.exe"

try {
    Push-Location $RepoRoot
    try {
        Write-Step "Building development $BinName"
        & go build -ldflags "-X github.com/InkyQuill/x-skills/internal/buildinfo.version=dev" -o $builtExe ./cmd/x-skills
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }

    $installedExe = Join-Path $InstallDir "$BinName.exe"
    $stagedExe = Join-Path $InstallDir ".$BinName.install.$PID.exe"
    Copy-Item -Force $builtExe $stagedExe
    try {
        if (Test-Path $installedExe) {
            Write-Step "existing $BinName found at $installedExe; replacing it"
            [System.IO.File]::Replace($stagedExe, $installedExe, $null)
        } else {
            [System.IO.File]::Move($stagedExe, $installedExe)
        }
    } catch [System.IO.IOException] {
        throw "replace $installedExe failed; close any running x-skills process and retry: $($_.Exception.Message)"
    } finally {
        Remove-Item -Force $stagedExe -ErrorAction SilentlyContinue
    }
    Install-XsShortcut
    Write-Host "installed $BinName to $installedExe"
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
