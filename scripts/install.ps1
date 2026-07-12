$ErrorActionPreference = "Stop"

$Repo = "InkyQuill/x-skills"
$BinName = "x-skills"
$InstallDir = if ($env:X_SKILLS_INSTALL_DIR) { $env:X_SKILLS_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }
$Version = if ($env:X_SKILLS_VERSION) { $env:X_SKILLS_VERSION } else { "latest" }

function Get-AssetName {
    $arch = switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        default { throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
    }

    if ($arch -eq "arm64") {
        throw "windows arm64 release artifacts are not published yet"
    }

    return "${BinName}_windows_${arch}.zip"
}

function Install-XsShortcut {
    if (Get-Command xs -ErrorAction SilentlyContinue) {
        Write-Host "xs already exists; leaving it unchanged"
        return
    }

    $shortcut = Join-Path $InstallDir "xs.cmd"
    if (Test-Path $shortcut) {
        Write-Host "$shortcut already exists; leaving it unchanged"
        return
    }

    "@echo off`r`n`"%~dp0x-skills.exe`" %*`r`n" | Set-Content -Encoding ASCII -NoNewline $shortcut
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$asset = Get-AssetName
if ($Version -eq "latest") {
    $url = "https://github.com/$Repo/releases/latest/download/$asset"
} else {
    $url = "https://github.com/$Repo/releases/download/$Version/$asset"
}

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
    $archive = Join-Path $tmp $asset
    Invoke-WebRequest -Uri $url -OutFile $archive
    Expand-Archive -LiteralPath $archive -DestinationPath $tmp -Force

    $exe = Join-Path $tmp "$BinName.exe"
    if (!(Test-Path $exe)) {
        throw "archive did not contain $BinName.exe"
    }

    Copy-Item -Force $exe (Join-Path $InstallDir "$BinName.exe")
    Install-XsShortcut
    Write-Host "installed $BinName to $(Join-Path $InstallDir "$BinName.exe")"
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
