//go:build linux

package config

import (
	"os"
	"path/filepath"
)

func defaultLogDir() string {
	return filepath.Join(xdgStateHome(), "battery-zen")
}

func platformConfigPaths() []string {
	return []string{
		// Local project config
		filepath.Join("internal", "config", "config.toml"),
		// User config
		filepath.Join(xdgConfigHome(), "battery-zen", "config.toml"),
		// System config
		"/etc/battery-zen/config.toml",
	}
}

func xdgConfigHome() string {
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		return value
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

func xdgStateHome() string {
	if value := os.Getenv("XDG_STATE_HOME"); value != "" {
		return value
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}
