
<div align="center">
<img src="assets/battery-zen.png" alt="Battery Zen Logo" width="120" />

# Battery Zen

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/github/license/Prajwal-Prathiksh/battery-zen?style=flat)](https://github.com/Prajwal-Prathiksh/battery-zen/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/Prajwal-Prathiksh/battery-zen?style=flat)](https://github.com/Prajwal-Prathiksh/battery-zen/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/Prajwal-Prathiksh/battery-zen)](https://goreportcard.com/report/github.com/Prajwal-Prathiksh/battery-zen)
![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20Windows-blue?style=flat)
[![Systemd](https://img.shields.io/badge/systemd-supported-green?style=flat)](https://systemd.io/)

*A zen-like Go toolkit for mindful battery monitoring, logging, and real-time visualization on Linux and Windows.*

<img src="assets/battery-zen-tui-v7-screenshot.png" alt="Battery Zen TUI Screenshot" width="480" />

</div>

## Features

- Battery and power monitoring
- Configurable logging intervals
- Automatic log rotation
- Systemd integration on Linux and per-user startup on Windows
- **Interactive TUI**: real-time charts, predictions, zoom/pan, cycle count
- **Screen-On Time (SOT) tracking** - estimates daily usage patterns
- **Suspend/shutdown detection** - identifies system sleep periods and battery drain
- **Weekly SOT visualization** - bar charts showing daily usage trends


## Install

### Linux

```bash
./install.sh
```

- This will:
  1. Build the binary
  2. Install the binary to `~/.local/bin/`
  3. Install, enable and start the systemd service, so `battery-zen` starts on boot and logs data in the background
  4. Create default config file at `~/.config/battery-zen/config.toml`

### Windows

Run PowerShell from the repository:

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

The script requires no administrator access. It builds `battery-zen.exe` with the Battery Zen icon, installs it under `%LOCALAPPDATA%\Programs\BatteryZen`, adds that directory to the user `PATH`, creates a Start Menu shortcut, starts the background logger, registers it to start when the current user signs in, and stores data under `%LOCALAPPDATA%\BatteryZen`.

> [!NOTE]
> Once installed on Linux, you can run `battery-zen` commands directly from your terminal, with tab completions for `bash` and `zsh`. On Windows, open a new terminal after installation, then run `battery-zen` directly.


## Usage

```bash
battery-zen          # Launch TUI
battery-zen tui      # Launch TUI explicitly
battery-zen status   # Show status
```

See [docs/TUI.md](docs/TUI.md) for advanced TUI features and controls.


## Service Management

### Linux

```bash
make status     # Service status
make logs       # View logs
make stop       # Stop service
make start      # Start service
make uninstall  # Remove everything
```

### Windows

```powershell
Get-Process battery-zen
Start-Process "$env:LOCALAPPDATA\Programs\BatteryZen\battery-zen.exe" -ArgumentList "run" -WindowStyle Hidden
Get-Process battery-zen | Stop-Process
```


## Configuration

Config files (TOML):
- Linux: [`internal/config/config.toml`](internal/config/config.toml), `~/.config/battery-zen/config.toml`, `/etc/battery-zen/config.toml`
- Windows: `%LOCALAPPDATA%\BatteryZen\config.toml`

The Windows installer creates the config only when it is missing, so reinstalling does not overwrite user changes. Restart the Windows background process after editing it:

```powershell
Get-Process battery-zen | Stop-Process
& wscript.exe "$env:LOCALAPPDATA\Programs\BatteryZen\start-battery-zen.vbs"
```

Key settings:
- `interval_secs`: Data logging frequency (default: 60s)
- `max_window_zoom`: Chart zoom limit in days (default: 10)
- Chart colors, log rotation, and timezone settings

See config file for all available options.



## Output

CSV logs:
- Linux telemetry: `~/.local/state/battery-zen/telemetry.csv`
- Linux lifecycle events: `~/.local/state/battery-zen/events.csv`
- Windows telemetry: `%LOCALAPPDATA%\BatteryZen\telemetry.csv`
- Windows lifecycle events: `%LOCALAPPDATA%\BatteryZen\events.csv`


## Analytics & Predictions

The TUI provides comprehensive battery analytics:

- **Charge/Discharge Rates**: Calculated using exponential weighted regression (recent data weighted higher)
- **Time Estimates**: Predicts time to full charge or empty based on current usage patterns
- **SOT Calculation**: Estimates active session time from explicit lifecycle transitions
- **Current Session**: Active time since last wake/boot
- **Daily Trends**: Bar chart showing SOT for the past 7 days
- **Suspend Detection**: Tracks sleep periods and battery drain during suspend

> **Note**: Suspend, resume, logoff, shutdown, and boot transitions are recorded automatically in `events.csv`. SOT remains an active-session proxy: time with the display off while the system remains awake is still counted.


## Background Installation

Linux service files are under [`systemd/`](systemd/). Windows installs a Startup-folder shortcut that explicitly invokes `wscript.exe`, with an HKCU `Run` fallback; both are configured automatically by `install.ps1`.


## Uninstall

### Linux

```bash
./uninstall.sh
```

### Windows

```powershell
powershell -ExecutionPolicy Bypass -File .\uninstall.ps1
```

Both scripts ask whether to remove configuration and logs, with **Yes as the default**. The Windows uninstaller also removes the Start Menu shortcut and the Battery Zen install directory from the user `PATH`; open a new terminal afterward. To preserve data without prompting:

```bash
./uninstall.sh --keep-data
```

```powershell
powershell -ExecutionPolicy Bypass -File .\uninstall.ps1 -KeepData
```


## Development

```bash
go build ./cmd/battery-zen
make clean
```

```powershell
go build -o battery-zen.exe ./cmd/battery-zen
```


## Configuration Reference

All configuration options available in config files:

### Core Settings
- `interval_secs = 60` - Data logging frequency in seconds
- `interval_secs_on_ac = 60` - Logging frequency when AC connected
- `timezone = "Local"` - Timezone for timestamps
- `log_dir = "~/.local/state/battery-zen"` - Directory for log files
- `log_file = "telemetry.csv"` - Name of the CSV log file
- `max_lines = 4000` - Maximum lines in log before rotation
- `trim_buffer = 100` - Lines to keep when trimming log
- `max_charge_percent = 100` - Maximum charge threshold for predictions

### TUI Settings
- `day_color_number = -1` - Terminal color for day data points (default foreground)
- `night_color_number = 234` - Terminal color for night data points (dark gray)
- `day_start_hour = 7` - Hour when day visualization starts (7 AM)
- `day_end_hour = 19` - Hour when night visualization starts (7 PM)
- `max_window_zoom = 10` - Maximum zoom window in days for charts
