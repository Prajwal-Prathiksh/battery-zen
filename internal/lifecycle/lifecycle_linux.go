//go:build linux

package lifecycle

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	loginManagerInterface = "org.freedesktop.login1.Manager"
	prepareForSleep       = loginManagerInterface + ".PrepareForSleep"
	prepareForShutdown    = loginManagerInterface + ".PrepareForShutdown"
	ackTimeout            = 2 * time.Second
)

type linuxMonitor struct {
	connection *dbus.Conn
	events     chan Event
	signals    chan *dbus.Signal
	done       chan struct{}
	inhibitor  *os.File
	closeOnce  sync.Once
	wait       sync.WaitGroup
}

func Start() (Monitor, error) {
	connection, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	path := dbus.ObjectPath("/org/freedesktop/login1")
	if err := connection.AddMatchSignal(
		dbus.WithMatchObjectPath(path),
		dbus.WithMatchInterface(loginManagerInterface),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		connection.Close()
		return nil, err
	}
	if err := connection.AddMatchSignal(
		dbus.WithMatchObjectPath(path),
		dbus.WithMatchInterface(loginManagerInterface),
		dbus.WithMatchMember("PrepareForShutdown"),
	); err != nil {
		connection.Close()
		return nil, err
	}
	monitor := &linuxMonitor{
		connection: connection,
		events:     make(chan Event, 16),
		signals:    make(chan *dbus.Signal, 16),
		done:       make(chan struct{}),
	}
	connection.Signal(monitor.signals)
	monitor.acquireInhibitor()
	monitor.wait.Add(1)
	go monitor.run()
	return monitor, nil
}

func (monitor *linuxMonitor) Events() <-chan Event {
	return monitor.events
}

func (monitor *linuxMonitor) Close() error {
	var err error
	monitor.closeOnce.Do(func() {
		close(monitor.done)
		monitor.connection.RemoveSignal(monitor.signals)
		err = monitor.connection.Close()
		monitor.wait.Wait()
		monitor.releaseInhibitor()
	})
	return err
}

func BootID() (string, error) {
	value, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	return "linux:" + strings.TrimSpace(string(value)), nil
}

func (monitor *linuxMonitor) run() {
	defer monitor.wait.Done()
	for {
		select {
		case <-monitor.done:
			return
		case signal := <-monitor.signals:
			if signal == nil || len(signal.Body) == 0 {
				continue
			}
			entering, ok := signal.Body[0].(bool)
			if !ok {
				continue
			}
			var kind Kind
			switch signal.Name {
			case prepareForSleep:
				if entering {
					kind = Suspend
				} else {
					kind = Resume
				}
			case prepareForShutdown:
				if entering {
					kind = Shutdown
				}
			}
			if kind == "" {
				continue
			}
			if kind == Resume {
				monitor.acquireInhibitor()
				monitor.emit(Event{Kind: kind, Time: time.Now()})
				continue
			}
			ack := make(chan struct{})
			if monitor.emit(Event{Kind: kind, Time: time.Now(), Ack: ack}) {
				select {
				case <-ack:
				case <-time.After(ackTimeout):
				case <-monitor.done:
				}
			}
			monitor.releaseInhibitor()
		}
	}
}

func (monitor *linuxMonitor) emit(event Event) bool {
	select {
	case monitor.events <- event:
		return true
	default:
		return false
	}
}

// A delay inhibitor makes logind wait until the event row is written before the user slice is frozen.
func (monitor *linuxMonitor) acquireInhibitor() {
	if monitor.inhibitor != nil {
		return
	}
	var fd dbus.UnixFD
	err := monitor.connection.Object("org.freedesktop.login1", "/org/freedesktop/login1").Call(
		loginManagerInterface+".Inhibit", 0,
		"sleep:shutdown", "Battery Zen", "Record battery state before sleep or shutdown", "delay",
	).Store(&fd)
	if err != nil {
		log.Printf("lifecycle inhibitor: %v", err)
		return
	}
	monitor.inhibitor = os.NewFile(uintptr(fd), "battery-zen-inhibitor")
}

func (monitor *linuxMonitor) releaseInhibitor() {
	if monitor.inhibitor != nil {
		monitor.inhibitor.Close()
		monitor.inhibitor = nil
	}
}
