//go:build linux

package lifecycle

import (
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
)

type linuxMonitor struct {
	connection *dbus.Conn
	events     chan Event
	signals    chan *dbus.Signal
	done       chan struct{}
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
			if kind != "" {
				select {
				case monitor.events <- Event{Kind: kind, Time: time.Now()}:
				default:
				}
			}
		}
	}
}
