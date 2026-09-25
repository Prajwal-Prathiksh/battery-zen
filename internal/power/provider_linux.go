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
	voltage, hasVoltage := sysfs.BatteryValue("voltage_now")
	if hasVoltage && voltage > 0 {
		reading.VoltageMV = linuxUint32(voltage / 1000)
	}
	designVoltage, ok := sysfs.BatteryValue("voltage_min_design")
	if !ok || designVoltage <= 0 {
		designVoltage = voltage
	}
	reading.RemainingCapacityMWh = linuxEnergy("energy_now", "charge_now", designVoltage)
	reading.FullChargedCapacityMWh = linuxEnergy("energy_full", "charge_full", designVoltage)
	reading.DesignCapacityMWh = linuxEnergy("energy_full_design", "charge_full_design", designVoltage)
	reading.RateMW = linuxRate(reading.State, voltage)
	return reading, Validate(reading)
}

// Energy attributes are µWh; charge attributes are µAh and need a voltage (µV) to become mWh.
func linuxEnergy(energyName, chargeName string, voltage int64) *uint32 {
	if energy, ok := sysfs.BatteryValue(energyName); ok && energy >= 0 {
		return linuxUint32(energy / 1000)
	}
	if charge, ok := sysfs.BatteryValue(chargeName); ok && charge >= 0 && voltage > 0 {
		return linuxUint32(charge * voltage / 1_000_000_000)
	}
	return nil
}

// Matches the Windows convention: negative while discharging.
func linuxRate(state State, voltage int64) *int32 {
	var milliwatts int64
	if power, ok := sysfs.BatteryValue("power_now"); ok {
		milliwatts = power / 1000
	} else if current, ok := sysfs.BatteryValue("current_now"); ok && voltage > 0 {
		milliwatts = current * voltage / 1_000_000_000
	} else {
		return nil
	}
	if milliwatts < 0 {
		milliwatts = -milliwatts
	}
	if state == StateDischarging {
		milliwatts = -milliwatts
	}
	value := int32(milliwatts)
	return &value
}

func linuxUint32(value int64) *uint32 {
	result := uint32(value)
	return &result
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
