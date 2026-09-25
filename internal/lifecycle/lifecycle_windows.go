//go:build windows

package lifecycle

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	deviceNotifyCallback       = 2
	powerEventSuspend          = 4
	powerEventResume           = 7
	powerEventResumeAuto       = 18
	ctrlLogoffEvent            = 5
	ctrlShutdownEvent          = 6
	systemTimeOfDayInformation = 3
)

var (
	lifecycleKernel32                = windows.NewLazySystemDLL("kernel32.dll")
	lifecyclePowrprof                = windows.NewLazySystemDLL("powrprof.dll")
	lifecycleNtdll                   = windows.NewLazySystemDLL("ntdll.dll")
	procPowerRegisterSuspendResume   = lifecyclePowrprof.NewProc("PowerRegisterSuspendResumeNotification")
	procPowerUnregisterSuspendResume = lifecyclePowrprof.NewProc("PowerUnregisterSuspendResumeNotification")
	procSetConsoleCtrlHandler        = lifecycleKernel32.NewProc("SetConsoleCtrlHandler")
	procNtQuerySystemInformation     = lifecycleNtdll.NewProc("NtQuerySystemInformation")
	windowsLifecycleSink             chan Event
	windowsLifecycleMu               sync.Mutex
	powerCallback                    = windows.NewCallback(powerNotificationCallback)
	consoleCallback                  = windows.NewCallback(consoleControlCallback)
)

type deviceNotifySubscribeParameters struct {
	Callback uintptr
	Context  uintptr
}

type systemTimeOfDay struct {
	BootTime          int64
	CurrentTime       int64
	TimeZoneBias      int64
	CurrentTimeZoneID uint32
	Reserved          uint32
	BootTimeBias      uint64
	SleepTimeBias     uint64
}

type windowsMonitor struct {
	events      chan Event
	powerHandle uintptr
	parameters  deviceNotifySubscribeParameters
}

func Start() (Monitor, error) {
	windowsLifecycleMu.Lock()
	defer windowsLifecycleMu.Unlock()
	if windowsLifecycleSink != nil {
		return nil, fmt.Errorf("lifecycle monitor already running")
	}
	monitor := &windowsMonitor{
		events:     make(chan Event, 16),
		parameters: deviceNotifySubscribeParameters{Callback: powerCallback},
	}
	windowsLifecycleSink = monitor.events
	result, _, _ := procPowerRegisterSuspendResume.Call(
		deviceNotifyCallback,
		uintptr(unsafe.Pointer(&monitor.parameters)),
		uintptr(unsafe.Pointer(&monitor.powerHandle)),
	)
	if result != 0 {
		windowsLifecycleSink = nil
		return nil, fmt.Errorf("PowerRegisterSuspendResumeNotification returned %d", result)
	}
	result, _, callErr := procSetConsoleCtrlHandler.Call(consoleCallback, 1)
	if result == 0 {
		procPowerUnregisterSuspendResume.Call(monitor.powerHandle)
		windowsLifecycleSink = nil
		return nil, fmt.Errorf("SetConsoleCtrlHandler: %v", callErr)
	}
	return monitor, nil
}

func (monitor *windowsMonitor) Events() <-chan Event {
	return monitor.events
}

func (monitor *windowsMonitor) Close() error {
	windowsLifecycleMu.Lock()
	windowsLifecycleSink = nil
	windowsLifecycleMu.Unlock()
	procSetConsoleCtrlHandler.Call(consoleCallback, 0)
	result, _, _ := procPowerUnregisterSuspendResume.Call(monitor.powerHandle)
	if result != 0 {
		return fmt.Errorf("PowerUnregisterSuspendResumeNotification returned %d", result)
	}
	return nil
}

func BootID() (string, error) {
	var information systemTimeOfDay
	var returned uint32
	status, _, _ := procNtQuerySystemInformation.Call(
		systemTimeOfDayInformation,
		uintptr(unsafe.Pointer(&information)),
		unsafe.Sizeof(information),
		uintptr(unsafe.Pointer(&returned)),
	)
	if int32(status) != 0 {
		return "", fmt.Errorf("NtQuerySystemInformation returned NTSTATUS 0x%08x", uint32(status))
	}
	return fmt.Sprintf("windows:%016x", uint64(information.BootTime)), nil
}

func powerNotificationCallback(_ uintptr, eventType uint32, _ uintptr) uintptr {
	var kind Kind
	switch eventType {
	case powerEventSuspend:
		kind = Suspend
	case powerEventResume, powerEventResumeAuto:
		kind = Resume
	default:
		return 0
	}
	emitWindowsEvent(Event{Kind: kind, Time: time.Now()})
	return 0
}

func consoleControlCallback(controlType uint32) uintptr {
	var kind Kind
	switch controlType {
	case ctrlLogoffEvent:
		kind = Logoff
	case ctrlShutdownEvent:
		kind = Shutdown
	default:
		return 0
	}
	ack := make(chan struct{})
	event := Event{Kind: kind, Time: time.Now(), Ack: ack}
	if emitWindowsEvent(event) {
		select {
		case <-ack:
		case <-time.After(2 * time.Second):
		}
	}
	return 1
}

func emitWindowsEvent(event Event) bool {
	windowsLifecycleMu.Lock()
	sink := windowsLifecycleSink
	windowsLifecycleMu.Unlock()
	if sink == nil {
		return false
	}
	select {
	case sink <- event:
		return true
	default:
		return false
	}
}
