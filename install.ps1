param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\dntproxy\bin",
    [string]$Repo = "dungnt1312/dntproxy"
)

$ErrorActionPreference = "Stop"

function Write-Info($Message) { Write-Host "[INFO] $Message" -ForegroundColor Cyan }
function Write-Ok($Message) { Write-Host "[OK] $Message" -ForegroundColor Green }
function Write-Err($Message) { Write-Host "[ERR] $Message" -ForegroundColor Red }

function Get-TargetArch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    switch ($arch) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch (supported: X64, Arm64)" }
    }
}

function Get-ReleaseUrl([string]$Arch) {
    $asset = "dntproxy-windows-$Arch.zip"
    if ($Version -eq "latest") {
        return "https://github.com/$Repo/releases/latest/download/$asset"
    }

    $tag = $Version
    if (-not $tag.StartsWith("v")) {
        $tag = "v$tag"
    }

    return "https://github.com/$Repo/releases/download/$tag/$asset"
}

try {
    $arch = Get-TargetArch
    $url = Get-ReleaseUrl -Arch $arch
    $tempRoot = Join-Path $env:TEMP ("dntproxy-install-" + [guid]::NewGuid().ToString("N"))
    $zipPath = Join-Path $tempRoot "dntproxy.zip"

    New-Item -ItemType Directory -Path $tempRoot | Out-Null

    Write-Info "Downloading dntproxy ($arch) from $url"
    Invoke-WebRequest -Uri $url -OutFile $zipPath

    Expand-Archive -LiteralPath $zipPath -DestinationPath $tempRoot -Force
    $exe = Get-ChildItem -Path $tempRoot -Recurse -Filter "dntproxy.exe" | Select-Object -First 1
    if (-not $exe) {
        throw "Binary dntproxy.exe not found in archive"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $destination = Join-Path $InstallDir "dntproxy.exe"
    Copy-Item -LiteralPath $exe.FullName -Destination $destination -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $normalized = ($userPath -split ";" | ForEach-Object { $_.Trim().TrimEnd("\\") })
    $targetPath = $InstallDir.TrimEnd("\\")
    if ($normalized -notcontains $targetPath) {
        $newPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $InstallDir } else { "$userPath;$InstallDir" }
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$InstallDir;$env:Path"
        Write-Info "Added $InstallDir to User PATH"
    }

    Write-Ok "Installed to $destination"
    Write-Info "Try: dntproxy.exe --help"
}
catch {
    Write-Err $_.Exception.Message
    exit 1
}
finally {
    if ($tempRoot -and (Test-Path $tempRoot)) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
}
