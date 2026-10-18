# Обратное действие для scripts/install.ps1: удаляет установленный бинарь и
# убирает его папку из PATH пользователя.
#
# Если install.ps1 запускался с $env:TRUNKLINE_INSTALL_DIR, задайте ту же
# переменную здесь, иначе бинарь не найдётся по умолчанному пути.

$ErrorActionPreference = "Stop"

$InstallDir = if ($env:TRUNKLINE_INSTALL_DIR) { $env:TRUNKLINE_INSTALL_DIR } else { Join-Path $env:USERPROFILE ".trunkline\bin" }
$BinPath = Join-Path $InstallDir "trunkline.exe"

if (Test-Path $BinPath) {
    Remove-Item $BinPath -Force
    Write-Host "Removed $BinPath"
    if ((Get-ChildItem $InstallDir -ErrorAction SilentlyContinue | Measure-Object).Count -eq 0) {
        Remove-Item $InstallDir -Force
    }
} else {
    Write-Host "$BinPath not found — nothing to remove."
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath) {
    $entries = $userPath -split ";" | Where-Object { $_ -ne "" -and $_ -ne $InstallDir }
    [Environment]::SetEnvironmentVariable("Path", ($entries -join ";"), "User")
    Write-Host "Removed $InstallDir from your user PATH."
}

Write-Host "Done."
