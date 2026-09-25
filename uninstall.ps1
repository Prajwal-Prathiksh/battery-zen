param(
    [switch]$KeepData
)

$ErrorActionPreference = "Stop"

function Normalize-PathEntry([string]$PathEntry) {
    if ([string]::IsNullOrWhiteSpace($PathEntry)) {
        return ""
    }
    return $PathEntry.Trim().Trim('"').TrimEnd([char[]]"\/")
}

$installDir = Join-Path $env:LOCALAPPDATA "Programs\BatteryZen"
$dataDir = Join-Path $env:LOCALAPPDATA "BatteryZen"
$targetExe = Join-Path $installDir "battery-zen.exe"
$startMenuDir = [Environment]::GetFolderPath("Programs")
$shortcutPath = Join-Path $startMenuDir "Battery Zen.lnk"
$startupDir = [Environment]::GetFolderPath("Startup")
$startupShortcutPath = Join-Path $startupDir "Battery Zen Background.lnk"
$legacyStartupLauncherPath = Join-Path $startupDir "Battery Zen Background.vbs"
$runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"

$removeData = -not $KeepData
if (-not $PSBoundParameters.ContainsKey("KeepData")) {
    $response = Read-Host "Remove Battery Zen configuration and logs? [Y/n]"
    $removeData = [string]::IsNullOrWhiteSpace($response) -or $response -match '^(y|yes)$'
}

if (Get-ItemProperty -Path $runKey -Name "BatteryZen" -ErrorAction SilentlyContinue) {
    Remove-ItemProperty -Path $runKey -Name "BatteryZen"
}

$processes = Get-Process -Name "battery-zen" -ErrorAction SilentlyContinue |
    Where-Object { $_.Path -eq $targetExe }
if ($null -ne $processes) {
    $processes | Stop-Process -Force -PassThru | Wait-Process
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$normalizedInstallDir = Normalize-PathEntry $installDir
$remainingPathEntries = @()
$pathRemoved = $false
foreach ($entry in @($userPath -split ';')) {
    if ((Normalize-PathEntry $entry) -ieq $normalizedInstallDir) {
        $pathRemoved = $true
    } else {
        $remainingPathEntries += $entry
    }
}
if ($pathRemoved) {
    [Environment]::SetEnvironmentVariable("Path", [string]::Join(';', $remainingPathEntries), "User")
}

if (Test-Path $shortcutPath) {
    Remove-Item $shortcutPath -Force
}
if (Test-Path $startupShortcutPath) {
    Remove-Item $startupShortcutPath -Force
}
if (Test-Path $legacyStartupLauncherPath) {
    Remove-Item $legacyStartupLauncherPath -Force
}

if (Test-Path $installDir) {
    Remove-Item $installDir -Recurse -Force
}

if ($removeData) {
    if (Test-Path $dataDir) {
        Remove-Item $dataDir -Recurse -Force
    }
    Write-Host "Removed Battery Zen configuration and logs."
} else {
    Write-Host "Preserved Battery Zen configuration and logs at $dataDir"
}

if ($pathRemoved) {
    Write-Host "Removed from user PATH: $installDir"
}
Write-Host "Battery Zen has been uninstalled for the current user."
Write-Host "Open a new terminal to receive the updated PATH."
