package lifecycle

import (
	"strings"
	"time"
)

type Kind string

const (
	Startup  Kind = "startup"
	Suspend  Kind = "suspend"
	Resume   Kind = "resume"
	Shutdown Kind = "shutdown"
	Restart  Kind = "restart"
	Logoff   Kind = "logoff"
)

type Event struct {
	Kind Kind
	Time time.Time
	Ack  chan struct{}
}

type Monitor interface {
	Events() <-chan Event
	Close() error
}

func BootChanged(previous, current string) bool {
	previousFamily, _, previousOK := strings.Cut(previous, ":")
	currentFamily, _, currentOK := strings.Cut(current, ":")
	return previousOK && currentOK && previousFamily == currentFamily && previous != current
}
