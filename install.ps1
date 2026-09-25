$ErrorActionPreference = "Stop"

function Normalize-PathEntry([string]$PathEntry) {
    if ([string]::IsNullOrWhiteSpace($PathEntry)) {
        return ""
    }
    return $PathEntry.Trim().Trim('"').TrimEnd([char[]]"\/")
}

Set-Location $PSScriptRoot
$goCommand = Get-Command go -ErrorAction SilentlyContinue
if ($null -ne $goCommand) {
    $go = $goCommand.Source
} else {
    $go = "C:\Program Files\Go\bin\go.exe"
}
if (-not (Test-Path $go)) {
    throw "Go was not found. Install Go or add it to PATH."
}

$installDir = Join-Path $env:LOCALAPPDATA "Programs\BatteryZen"
$dataDir = Join-Path $env:LOCALAPPDATA "BatteryZen"
$targetExe = Join-Path $installDir "battery-zen.exe"
$launcherPath = Join-Path $installDir "start-battery-zen.vbs"
$configPath = Join-Path $dataDir "config.toml"
$startMenuDir = [Environment]::GetFolderPath("Programs")
$shortcutPath = Join-Path $startMenuDir "Battery Zen.lnk"
$startupDir = [Environment]::GetFolderPath("Startup")
$startupShortcutPath = Join-Path $startupDir "Battery Zen Background.lnk"
$legacyStartupLauncherPath = Join-Path $startupDir "Battery Zen Background.vbs"
$temporaryExe = Join-Path $env:TEMP "battery-zen-install-$PID.exe"

New-Item -ItemType Directory -Path $installDir, $dataDir, $startupDir -Force | Out-Null

$writeConfig = -not (Test-Path $configPath)
if ($writeConfig) {
    $normalizedDataDir = $dataDir.Replace("\", "/")
    $config = Get-Content (Join-Path $PSScriptRoot "internal\config\config.toml") -Raw
    $config = $config -replace '(?m)^log_dir\s*=.*$', "log_dir = `"$normalizedDataDir`"  # Directory for log files"
} else {
    $config = Get-Content $configPath -Raw
}
$updatedConfig = $config -replace '(?m)^suspend_gap_minutes\s*=.*\r?\n?', ''
if ($writeConfig -or $updatedConfig -ne $config) {
    Set-Content -Path $configPath -Value $updatedConfig -Encoding UTF8
}

try {
    & $go build -trimpath -ldflags "-s -w" -o $temporaryExe ./cmd/battery-zen
    if ($LASTEXITCODE -ne 0) {
        throw "Battery Zen build failed."
    }

    Get-Process -Name "battery-zen" -ErrorAction SilentlyContinue |
        Where-Object { $_.Path -eq $targetExe } |
        Stop-Process -Force

    Copy-Item $temporaryExe $targetExe -Force

    $shell = New-Object -ComObject WScript.Shell
    $shortcut = $shell.CreateShortcut($shortcutPath)
    $shortcut.TargetPath = $targetExe
    $shortcut.WorkingDirectory = $dataDir
    $shortcut.IconLocation = "$targetExe,0"
    $shortcut.Description = "Battery Zen battery monitor"
    $shortcut.Save()

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $pathEntries = @($userPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
    $normalizedInstallDir = Normalize-PathEntry $installDir
    $pathExists = $pathEntries | Where-Object { (Normalize-PathEntry $_) -ieq $normalizedInstallDir }
    $pathAdded = $false
    if (-not $pathExists) {
        $pathEntries += $installDir
        [Environment]::SetEnvironmentVariable("Path", [string]::Join(';', $pathEntries), "User")
        $pathAdded = $true
    }

    $launcher = @"
Set shell = CreateObject("WScript.Shell")
shell.Run """$targetExe"" run", 0, False
"@
    Set-Content -Path $launcherPath -Value $launcher -Encoding ASCII
    if (Test-Path $legacyStartupLauncherPath) {
        Remove-Item $legacyStartupLauncherPath -Force
    }
    $startupShortcut = $shell.CreateShortcut($startupShortcutPath)
    $startupShortcut.TargetPath = Join-Path $env:SystemRoot "System32\wscript.exe"
    $startupShortcut.Arguments = "`"$launcherPath`""
    $startupShortcut.WorkingDirectory = $installDir
    $startupShortcut.IconLocation = "$targetExe,0"
    $startupShortcut.WindowStyle = 7
    $startupShortcut.Description = "Start Battery Zen background logger"
    $startupShortcut.Save()

    $runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
    New-Item -Path $runKey -Force | Out-Null
    New-ItemProperty -Path $runKey -Name "BatteryZen" -Value "wscript.exe `"$launcherPath`"" -PropertyType String -Force | Out-Null

    Start-Process -FilePath $targetExe -ArgumentList "run" -WindowStyle Hidden

    Write-Host "Battery Zen installed for the current user."
    Write-Host "Executable: $targetExe"
    Write-Host "Battery data: $dataDir"
    Write-Host "Configuration: $configPath"
    Write-Host "Start Menu shortcut: $shortcutPath"
    Write-Host "Login startup shortcut: $startupShortcutPath"
    if ($pathAdded) {
        Write-Host "Added to user PATH: $installDir"
    } else {
        Write-Host "Already present in user PATH: $installDir"
    }
    Write-Host "Battery Zen will start automatically when this user signs in."
    Write-Host "Open a new terminal before running battery-zen by name."
} finally {
    Remove-Item $temporaryExe -Force -ErrorAction SilentlyContinue
}
