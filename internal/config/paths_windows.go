//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func defaultLogDir() string {
	return filepath.Join(localAppDataDir(), "BatteryZen")
}

func platformConfigPaths() []string {
	return []string{filepath.Join(defaultLogDir(), "config.toml")}
}

func localAppDataDir() string {
	if value := os.Getenv("LOCALAPPDATA"); value != "" {
		return value
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local")
}
