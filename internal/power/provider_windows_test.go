//go:build windows

package power

import (
	"testing"
	"unsafe"
)

func TestWindowsStructureSizes(t *testing.T) {
	checks := map[string]struct {
		got  uintptr
		want uintptr
	}{
		"systemPowerStatus":       {unsafe.Sizeof(systemPowerStatus{}), 12},
		"systemBatteryState":      {unsafe.Sizeof(systemBatteryState{}), 32},
		"batteryQueryInformation": {unsafe.Sizeof(batteryQueryInformation{}), 12},
		"batteryWaitStatus":       {unsafe.Sizeof(batteryWaitStatus{}), 20},
		"batteryInformationData":  {unsafe.Sizeof(batteryInformationData{}), 36},
		"batteryStatusData":       {unsafe.Sizeof(batteryStatusData{}), 16},
	}
	for name, check := range checks {
		if check.got != check.want {
			t.Errorf("%s size = %d, want %d", name, check.got, check.want)
		}
	}
}

func TestWindowsProviderWhenBatteryAvailable(t *testing.T) {
	reading, err := New().Read()
	if err != nil {
		t.Skipf("battery unavailable: %v", err)
	}
	if err := Validate(reading); err != nil {
		t.Fatal(err)
	}
}
