//go:build windows

package power

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	digcfPresent                       = 0x00000002
	digcfDeviceInterface               = 0x00000010
	ioctlBatteryQueryTag               = 0x00294040
	ioctlBatteryQueryInformation       = 0x00294044
	ioctlBatteryQueryStatus            = 0x0029404c
	batteryInformation                 = 0
	batterySystemBattery               = 0x80000000
	batteryCapacityRelative            = 0x40000000
	batteryPowerOnLine                 = 0x00000001
	batteryDischarging                 = 0x00000002
	batteryCharging                    = 0x00000004
	batteryUnknownCapacity             = 0xffffffff
	batteryUnknownVoltage              = 0xffffffff
	batteryUnknownRate           int32 = -2147483648
)

var (
	kernel32                        = windows.NewLazySystemDLL("kernel32.dll")
	powrprof                        = windows.NewLazySystemDLL("powrprof.dll")
	setupapi                        = windows.NewLazySystemDLL("setupapi.dll")
	procGetSystemPowerStatus        = kernel32.NewProc("GetSystemPowerStatus")
	procCallNtPowerInformation      = powrprof.NewProc("CallNtPowerInformation")
	procSetupDiGetClassDevsW        = setupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces = setupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetInterfaceDetailW  = setupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDeviceInfo    = setupapi.NewProc("SetupDiDestroyDeviceInfoList")
	batteryClassGUID                = windows.GUID{Data1: 0x72631e54, Data2: 0x78a4, Data3: 0x11d0, Data4: [8]byte{0xbc, 0xf7, 0x00, 0xaa, 0x00, 0xb7, 0xb3, 0x2a}}
)

type windowsProvider struct{}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

type systemBatteryState struct {
	ACOnLine          byte
	BatteryPresent    byte
	Charging          byte
	Discharging       byte
	Spare             [4]byte
	MaxCapacity       uint32
	RemainingCapacity uint32
	Rate              uint32
	EstimatedTime     uint32
	DefaultAlert1     uint32
	DefaultAlert2     uint32
}

type spDeviceInterfaceData struct {
	Size               uint32
	InterfaceClassGUID windows.GUID
	Flags              uint32
	Reserved           uintptr
}

type batteryQueryInformation struct {
	BatteryTag       uint32
	InformationLevel uint32
	AtRate           int32
}

type batteryWaitStatus struct {
	BatteryTag   uint32
	Timeout      uint32
	PowerState   uint32
	LowCapacity  uint32
	HighCapacity uint32
}

type batteryInformationData struct {
	Capabilities        uint32
	Technology          byte
	Reserved            [3]byte
	Chemistry           [4]byte
	DesignedCapacity    uint32
	FullChargedCapacity uint32
	DefaultAlert1       uint32
	DefaultAlert2       uint32
	CriticalBias        uint32
	CycleCount          uint32
}

type batteryStatusData struct {
	PowerState uint32
	Capacity   uint32
	Voltage    uint32
	Rate       int32
}

type batteryDetail struct {
	Online               bool
	Charging             bool
	Discharging          bool
	RemainingCapacityMWh *uint32
	FullCapacityMWh      *uint32
	DesignCapacityMWh    *uint32
	RateMW               *int32
	VoltageMV            *uint32
	CycleCount           *uint32
}

func New() Provider {
	return windowsProvider{}
}

func (windowsProvider) Read() (Reading, error) {
	reading := Reading{Percent: -1, State: StateUnknown}
	basic, basicErr := readSystemPowerStatus()
	if basicErr == nil {
		reading.ACConnected = basic.ACLineStatus == 1
		if basic.BatteryLifePercent != 255 {
			reading.Percent = int(basic.BatteryLifePercent)
		}
		reading.State = stateFromWindowsFlags(reading.ACConnected, basic.BatteryFlag, reading.Percent)
	}

	details, detailErr := enumerateBatteryDetails()
	if detailErr == nil && len(details) > 0 {
		mergeBatteryDetails(&reading, details)
	} else if aggregate, err := readAggregateBatteryState(); err == nil && aggregate.BatteryPresent != 0 {
		mergeAggregateBatteryState(&reading, aggregate)
	}
	if reading.Percent < 0 {
		if basicErr != nil && detailErr != nil {
			return Reading{}, fmt.Errorf("Windows battery APIs failed: %v; %v", basicErr, detailErr)
		}
		return Reading{}, errors.New("battery percentage unavailable")
	}
	return reading, Validate(reading)
}

func readSystemPowerStatus() (systemPowerStatus, error) {
	var status systemPowerStatus
	result, _, callErr := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&status)))
	if result == 0 {
		return systemPowerStatus{}, windowsCallError("GetSystemPowerStatus", callErr)
	}
	return status, nil
}

func readAggregateBatteryState() (systemBatteryState, error) {
	var state systemBatteryState
	result, _, _ := procCallNtPowerInformation.Call(5, 0, 0, uintptr(unsafe.Pointer(&state)), unsafe.Sizeof(state))
	if int32(result) != 0 {
		return systemBatteryState{}, fmt.Errorf("CallNtPowerInformation returned NTSTATUS 0x%08x", uint32(result))
	}
	return state, nil
}

func mergeAggregateBatteryState(reading *Reading, state systemBatteryState) {
	reading.ACConnected = state.ACOnLine != 0
	reading.RemainingCapacityMWh = optionalUint32(state.RemainingCapacity, batteryUnknownCapacity)
	reading.FullChargedCapacityMWh = optionalUint32(state.MaxCapacity, batteryUnknownCapacity)
	reading.RateMW = optionalInt32(int32(state.Rate), batteryUnknownRate)
	if reading.Percent < 0 {
		reading.Percent = capacityPercent(reading.RemainingCapacityMWh, reading.FullChargedCapacityMWh)
	}
	switch {
	case state.Charging != 0:
		reading.State = StateCharging
	case state.Discharging != 0:
		reading.State = StateDischarging
	case reading.ACConnected && reading.Percent >= 100:
		reading.State = StateFull
	case reading.ACConnected:
		reading.State = StateIdle
	default:
		reading.State = StateUnknown
	}
}

func mergeBatteryDetails(reading *Reading, details []batteryDetail) {
	var remaining, full, design uint64
	var rate int64
	var hasRemaining, hasFull, hasDesign, hasRate bool
	for _, detail := range details {
		reading.ACConnected = reading.ACConnected || detail.Online
		if detail.Charging {
			reading.State = StateCharging
		} else if detail.Discharging && reading.State != StateCharging {
			reading.State = StateDischarging
		}
		if detail.RemainingCapacityMWh != nil {
			remaining += uint64(*detail.RemainingCapacityMWh)
			hasRemaining = true
		}
		if detail.FullCapacityMWh != nil {
			full += uint64(*detail.FullCapacityMWh)
			hasFull = true
		}
		if detail.DesignCapacityMWh != nil {
			design += uint64(*detail.DesignCapacityMWh)
			hasDesign = true
		}
		if detail.RateMW != nil {
			rate += int64(*detail.RateMW)
			hasRate = true
		}
		if reading.VoltageMV == nil && detail.VoltageMV != nil {
			reading.VoltageMV = detail.VoltageMV
		}
		if detail.CycleCount != nil && (reading.CycleCount == nil || *detail.CycleCount > *reading.CycleCount) {
			reading.CycleCount = detail.CycleCount
		}
	}
	reading.RemainingCapacityMWh = boundedUint32(remaining, hasRemaining)
	reading.FullChargedCapacityMWh = boundedUint32(full, hasFull)
	reading.DesignCapacityMWh = boundedUint32(design, hasDesign)
	reading.RateMW = boundedInt32(rate, hasRate)
	if reading.Percent < 0 {
		reading.Percent = capacityPercent(reading.RemainingCapacityMWh, reading.FullChargedCapacityMWh)
	}
	if reading.State == StateUnknown {
		if reading.ACConnected && reading.Percent >= 100 {
			reading.State = StateFull
		} else if reading.ACConnected {
			reading.State = StateIdle
		} else {
			reading.State = StateDischarging
		}
	}
}

func enumerateBatteryDetails() ([]batteryDetail, error) {
	handle, _, callErr := procSetupDiGetClassDevsW.Call(uintptr(unsafe.Pointer(&batteryClassGUID)), 0, 0, digcfPresent|digcfDeviceInterface)
	if windows.Handle(handle) == windows.InvalidHandle {
		return nil, windowsCallError("SetupDiGetClassDevsW", callErr)
	}
	defer procSetupDiDestroyDeviceInfo.Call(handle)

	var details []batteryDetail
	for index := uint32(0); ; index++ {
		data := spDeviceInterfaceData{Size: uint32(unsafe.Sizeof(spDeviceInterfaceData{}))}
		result, _, enumErr := procSetupDiEnumDeviceInterfaces.Call(handle, 0, uintptr(unsafe.Pointer(&batteryClassGUID)), uintptr(index), uintptr(unsafe.Pointer(&data)))
		if result == 0 {
			if errors.Is(enumErr, windows.ERROR_NO_MORE_ITEMS) {
				break
			}
			return details, windowsCallError("SetupDiEnumDeviceInterfaces", enumErr)
		}
		path, err := batteryDevicePath(handle, &data)
		if err != nil {
			continue
		}
		detail, err := queryBatteryDetail(path)
		if err == nil {
			details = append(details, detail)
		}
	}
	return details, nil
}

func batteryDevicePath(deviceInfoSet uintptr, data *spDeviceInterfaceData) (string, error) {
	var required uint32
	procSetupDiGetInterfaceDetailW.Call(deviceInfoSet, uintptr(unsafe.Pointer(data)), 0, 0, uintptr(unsafe.Pointer(&required)), 0)
	if required == 0 {
		return "", errors.New("battery device path unavailable")
	}
	buffer := make([]byte, required)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		*(*uint32)(unsafe.Pointer(&buffer[0])) = 8
	} else {
		*(*uint32)(unsafe.Pointer(&buffer[0])) = 6
	}
	result, _, callErr := procSetupDiGetInterfaceDetailW.Call(deviceInfoSet, uintptr(unsafe.Pointer(data)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(required), uintptr(unsafe.Pointer(&required)), 0)
	if result == 0 {
		return "", windowsCallError("SetupDiGetDeviceInterfaceDetailW", callErr)
	}
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(&buffer[4]))), nil
}

func queryBatteryDetail(path string) (batteryDetail, error) {
	pathPointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return batteryDetail{}, err
	}
	var handle windows.Handle
	for _, access := range []uint32{windows.GENERIC_READ | windows.GENERIC_WRITE, windows.GENERIC_READ, 0} {
		handle, err = windows.CreateFile(pathPointer, access, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err == nil {
			break
		}
	}
	if err != nil {
		return batteryDetail{}, err
	}
	defer windows.CloseHandle(handle)

	var tag uint32
	var timeout uint32
	if err := batteryDeviceControl(handle, ioctlBatteryQueryTag, unsafe.Pointer(&timeout), uint32(unsafe.Sizeof(timeout)), unsafe.Pointer(&tag), uint32(unsafe.Sizeof(tag))); err != nil {
		return batteryDetail{}, err
	}
	if tag == 0 {
		return batteryDetail{}, errors.New("battery returned tag 0")
	}
	var info batteryInformationData
	query := batteryQueryInformation{BatteryTag: tag, InformationLevel: batteryInformation}
	if err := batteryDeviceControl(handle, ioctlBatteryQueryInformation, unsafe.Pointer(&query), uint32(unsafe.Sizeof(query)), unsafe.Pointer(&info), uint32(unsafe.Sizeof(info))); err != nil {
		return batteryDetail{}, err
	}
	wait := batteryWaitStatus{BatteryTag: tag, HighCapacity: batteryUnknownCapacity}
	var status batteryStatusData
	if err := batteryDeviceControl(handle, ioctlBatteryQueryStatus, unsafe.Pointer(&wait), uint32(unsafe.Sizeof(wait)), unsafe.Pointer(&status), uint32(unsafe.Sizeof(status))); err != nil {
		return batteryDetail{}, err
	}
	detail := batteryDetail{
		Online:      status.PowerState&batteryPowerOnLine != 0,
		Charging:    status.PowerState&batteryCharging != 0,
		Discharging: status.PowerState&batteryDischarging != 0,
		RateMW:      optionalInt32(status.Rate, batteryUnknownRate),
		VoltageMV:   optionalUint32(status.Voltage, batteryUnknownVoltage),
		CycleCount:  optionalUint32(info.CycleCount, batteryUnknownCapacity),
	}
	if info.Capabilities&batterySystemBattery != 0 && info.Capabilities&batteryCapacityRelative == 0 {
		detail.RemainingCapacityMWh = optionalUint32(status.Capacity, batteryUnknownCapacity)
		detail.FullCapacityMWh = optionalUint32(info.FullChargedCapacity, batteryUnknownCapacity)
		detail.DesignCapacityMWh = optionalUint32(info.DesignedCapacity, batteryUnknownCapacity)
	}
	return detail, nil
}

func batteryDeviceControl(handle windows.Handle, code uint32, input unsafe.Pointer, inputSize uint32, output unsafe.Pointer, outputSize uint32) error {
	var returned uint32
	var inputBuffer, outputBuffer *byte
	if input != nil {
		inputBuffer = (*byte)(input)
	}
	if output != nil {
		outputBuffer = (*byte)(output)
	}
	return windows.DeviceIoControl(handle, code, inputBuffer, inputSize, outputBuffer, outputSize, &returned, nil)
}

func stateFromWindowsFlags(acConnected bool, batteryFlag byte, percent int) State {
	switch {
	case batteryFlag&8 != 0:
		return StateCharging
	case !acConnected:
		return StateDischarging
	case percent >= 100:
		return StateFull
	case acConnected:
		return StateIdle
	default:
		return StateUnknown
	}
}

func capacityPercent(remaining, full *uint32) int {
	if remaining == nil || full == nil || *full == 0 {
		return -1
	}
	percent := (uint64(*remaining)*100 + uint64(*full)/2) / uint64(*full)
	if percent > 100 {
		percent = 100
	}
	return int(percent)
}

func optionalUint32(value, unknown uint32) *uint32 {
	if value == unknown {
		return nil
	}
	result := value
	return &result
}

func optionalInt32(value, unknown int32) *int32 {
	if value == unknown {
		return nil
	}
	result := value
	return &result
}

func boundedUint32(value uint64, present bool) *uint32 {
	if !present || value > uint64(^uint32(0)) {
		return nil
	}
	result := uint32(value)
	return &result
}

func boundedInt32(value int64, present bool) *int32 {
	if !present || value < -2147483648 || value > 2147483647 {
		return nil
	}
	result := int32(value)
	return &result
}

func windowsCallError(name string, err error) error {
	if err == nil || errors.Is(err, windows.ERROR_SUCCESS) {
		return fmt.Errorf("%s failed", name)
	}
	return fmt.Errorf("%s: %w", name, err)
}
