# Собирает trunkline из этого чекаута и кладёт в свою папку — не полагаясь
# на GOPATH/GOBIN — затем интерактивно предлагает добавить её в PATH
# пользователя (Environment "User", без прав администратора).
#
# Переопределить папку установки: $env:TRUNKLINE_INSTALL_DIR = "C:\custom\path"
#
# Обратное действие: scripts/uninstall.ps1

$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $PSScriptRoot
$InstallDir = if ($env:TRUNKLINE_INSTALL_DIR) { $env:TRUNKLINE_INSTALL_DIR } else { Join-Path $env:USERPROFILE ".trunkline\bin" }
$BinPath = Join-Path $InstallDir "trunkline.exe"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

Write-Host "Building trunkline into $InstallDir ..."
Push-Location $RootDir
try {
    go build -trimpath -o $BinPath ./cmd/trunkline
} finally {
    Pop-Location
}

$Version = (Get-Content (Join-Path $RootDir "VERSION") -ErrorAction SilentlyContinue)
Write-Host "Installed trunkline $Version -> $BinPath"

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$pathEntries = @()
if ($userPath) { $pathEntries = $userPath -split ";" }

if ($pathEntries -contains $InstallDir) {
    Write-Host "$InstallDir is already on PATH — nothing else to do."
    exit 0
}

Write-Host ""
Write-Host "$InstallDir is not on your PATH."

if (-not [Environment]::UserInteractive) {
    Write-Host "Add it manually, e.g. in PowerShell:"
    Write-Host "  [Environment]::SetEnvironmentVariable('Path', `"`$env:Path;$InstallDir`", 'User')"
    exit 0
}

$answer = Read-Host "Add it to your user PATH now? [Y/n]"
if ($answer -match '^[nN]') {
    Write-Host "Skipped. Add it yourself later, or re-run this script."
    exit 0
}

$newPath = if ([string]::IsNullOrEmpty($userPath)) { $InstallDir } else { "$userPath;$InstallDir" }
[Environment]::SetEnvironmentVariable("Path", $newPath, "User")
Write-Host "Added $InstallDir to your user PATH. Open a new terminal for it to take effect."
