//go:build linux

package sysfs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func readFirst(glob string) (string, bool) {
	matches, _ := filepath.Glob(glob)
	for _, p := range matches {
		if b, err := os.ReadFile(p); err == nil {
			return strings.TrimSpace(string(b)), true
		}
	}
	return "", false
}

func BatteryPercent() (int, bool) {
	if s, ok := readFirst("/sys/class/power_supply/BAT*/capacity"); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return v, true
		}
	}
	return 0, false
}

// Returns true if AC online; falls back to BAT status
func ACOnline() bool {
	if s, ok := readFirst("/sys/class/power_supply/AC*/online"); ok {
		return s == "1"
	}
	if s, ok := readFirst("/sys/class/power_supply/ACAD*/online"); ok {
		return s == "1"
	}
	if s, ok := readFirst("/sys/class/power_supply/ADP*/online"); ok {
		return s == "1"
	}
	// Fallback: infer from status
	if s, ok := readFirst("/sys/class/power_supply/BAT*/status"); ok {
		switch s {
		case "Charging", "Full":
			return true
		}
	}
	return false
}

func BatteryStatus() string {
	status, _ := readFirst("/sys/class/power_supply/BAT*/status")
	return status
}

func BatteryCycleCount() (int, bool) {
	if s, ok := readFirst("/sys/class/power_supply/BAT*/cycle_count"); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return v, true
		}
	}
	return 0, false
}

// BatteryValue reads an integer attribute such as energy_now (µWh) or voltage_now (µV).
func BatteryValue(name string) (int64, bool) {
	if s, ok := readFirst("/sys/class/power_supply/BAT*/" + name); ok {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v, true
		}
	}
	return 0, false
}
