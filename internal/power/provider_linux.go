//go:build linux

package power

import (
	"fmt"
	"strings"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/sysfs"
)

type linuxProvider struct{}

func New() Provider {
	return linuxProvider{}
}

func (linuxProvider) Read() (Reading, error) {
	percent, ok := sysfs.BatteryPercent()
	if !ok {
		return Reading{}, fmt.Errorf("battery percentage not found")
	}
	reading := Reading{
		ACConnected: sysfs.ACOnline(),
		Percent:     percent,
		State:       stateFromLinuxStatus(sysfs.BatteryStatus()),
	}
	if reading.State == StateUnknown {
		if reading.ACConnected {
			reading.State = StateCharging
		} else {
			reading.State = StateDischarging
		}
	}
	if cycleCount, ok := sysfs.BatteryCycleCount(); ok && cycleCount >= 0 {
		value := uint32(cycleCount)
		reading.CycleCount = &value
	}
	return reading, Validate(reading)
}

func stateFromLinuxStatus(status string) State {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "charging":
		return StateCharging
	case "discharging":
		return StateDischarging
	case "full":
		return StateFull
	case "not charging":
		return StateIdle
	default:
		return StateUnknown
	}
}
