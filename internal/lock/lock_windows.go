//go:build windows

package lock

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

type namedMutex struct {
	name   string
	handle windows.Handle
}

func newPlatformLock(path string) Instance {
	hash := sha256.Sum256([]byte(path))
	return &namedMutex{name: fmt.Sprintf(`Local\BatteryZen-%x`, hash[:8])}
}

func (mutex *namedMutex) Acquire() (bool, error) {
	name, err := windows.UTF16PtrFromString(mutex.name)
	if err != nil {
		return false, err
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(handle)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	mutex.handle = handle
	return true, nil
}

func (mutex *namedMutex) Release() {
	if mutex.handle != 0 {
		_ = windows.CloseHandle(mutex.handle)
		mutex.handle = 0
	}
}
