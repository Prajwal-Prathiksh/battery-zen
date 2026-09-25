package power

import "fmt"

type State string

const (
	StateUnknown     State = "unknown"
	StateCharging    State = "charging"
	StateDischarging State = "discharging"
	StateFull        State = "full"
	StateIdle        State = "idle"
)

type Reading struct {
	ACConnected            bool
	Percent                int
	State                  State
	RemainingCapacityMWh   *uint32
	FullChargedCapacityMWh *uint32
	DesignCapacityMWh      *uint32
	RateMW                 *int32
	VoltageMV              *uint32
	CycleCount             *uint32
}

type Provider interface {
	Read() (Reading, error)
}

func Validate(reading Reading) error {
	if reading.Percent < 0 || reading.Percent > 100 {
		return fmt.Errorf("battery percentage out of range: %d", reading.Percent)
	}
	return nil
}
