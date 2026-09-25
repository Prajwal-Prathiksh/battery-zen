//go:build windows

package lifecycle

import (
	"strings"
	"testing"
)

func TestWindowsBootID(t *testing.T) {
	bootID, err := BootID()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("boot ID: %s", bootID)
	if !strings.HasPrefix(bootID, "windows:") {
		t.Fatalf("boot ID = %q, want windows prefix", bootID)
	}
}

func TestWindowsMonitorRegistration(t *testing.T) {
	monitor, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	powerNotificationCallback(0, powerEventSuspend, 0)
	event := <-monitor.Events()
	if event.Kind != Suspend {
		t.Fatalf("event = %q, want suspend", event.Kind)
	}
	callbackDone := make(chan struct{})
	go func() {
		consoleControlCallback(ctrlShutdownEvent)
		close(callbackDone)
	}()
	event = <-monitor.Events()
	if event.Kind != Shutdown || event.Ack == nil {
		t.Fatalf("event = %+v, want acknowledged shutdown", event)
	}
	close(event.Ack)
	<-callbackDone
	if err := monitor.Close(); err != nil {
		t.Fatal(err)
	}
}
