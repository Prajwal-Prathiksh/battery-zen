//go:build windows

package config

import (
	"path/filepath"
	"testing"
)

func TestWindowsDefaultPaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", root)
	cfg := Defaults()
	expectedDir := filepath.Join(root, "BatteryZen")
	if cfg.LogDir != expectedDir {
		t.Fatalf("LogDir = %q, want %q", cfg.LogDir, expectedDir)
	}
	paths := platformConfigPaths()
	if len(paths) != 1 || paths[0] != filepath.Join(expectedDir, "config.toml") {
		t.Fatalf("config paths = %v", paths)
	}
}
