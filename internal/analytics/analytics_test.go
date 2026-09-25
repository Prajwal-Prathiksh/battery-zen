package analytics

import (
	"testing"
	"time"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/lifecycle"
)

func TestExplicitSuspendEvents(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	rows := []Row{
		{T: start, Batt: 80},
		{T: start.Add(20 * time.Minute), Batt: 77},
	}
	records := []lifecycle.Record{
		{Time: start, Kind: lifecycle.Startup, BootID: "test:boot-a", Battery: 80},
		{Time: start.Add(5 * time.Minute), Kind: lifecycle.Suspend, BootID: "test:boot-a", Battery: 79},
		{Time: start.Add(15 * time.Minute), Kind: lifecycle.Resume, BootID: "test:boot-a", Battery: 78},
	}
	result := CalculateScreenOnTime(rows, records)
	if len(result.SuspendEvents) != 1 || result.SuspendEvents[0].Kind != lifecycle.Suspend {
		t.Fatalf("unexpected events: %+v", result.SuspendEvents)
	}
	if result.SuspendTime != 10*time.Minute || result.TotalActiveTime != 10*time.Minute || result.LastActiveSession != 5*time.Minute {
		t.Fatalf("unexpected screen-on result: %+v", result)
	}
}

func TestSameBootProcessRestartDoesNotResetSession(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	rows := []Row{{T: start, Batt: 80}, {T: start.Add(20 * time.Minute), Batt: 78}}
	records := []lifecycle.Record{
		{Time: start, Kind: lifecycle.Startup, BootID: "test:boot-a", Battery: 80},
		{Time: start.Add(10 * time.Minute), Kind: lifecycle.Startup, BootID: "test:boot-a", Battery: 79},
	}
	result := CalculateScreenOnTime(rows, records)
	if result.LastActiveSession != 20*time.Minute {
		t.Fatalf("last active session = %v, want 20m", result.LastActiveSession)
	}
}

func TestBootChangeDetectsRestart(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	rows := []Row{
		{T: start, Batt: 80},
		{T: start.Add(9 * time.Minute), Batt: 78},
		{T: start.Add(20 * time.Minute), Batt: 77},
	}
	records := []lifecycle.Record{
		{Time: start, Kind: lifecycle.Startup, BootID: "test:boot-a", Battery: 80},
		{Time: start.Add(20 * time.Minute), Kind: lifecycle.Startup, BootID: "test:boot-b", Battery: 77},
	}
	events := DetectSuspendEvents(rows, records)
	if len(events) != 1 || events[0].Kind != lifecycle.Restart {
		t.Fatalf("unexpected events: %+v", events)
	}
	if events[0].StartTime != rows[1].T || events[0].Duration != 11*time.Minute {
		t.Fatalf("unexpected restart event: %+v", events[0])
	}
}

func TestParseExtendedCSVRows(t *testing.T) {
	rows := [][]string{
		{"timestamp", "ac_connected", "battery_life", "state", "remaining_capacity_mwh", "rate_mw"},
		{"2026-01-01T00:00:00Z", "0", "50", "discharging", "40000", "-12000"},
	}
	parsed, err := ParseCSVRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].AC || parsed[0].Batt != 50 {
		t.Fatalf("unexpected rows: %+v", parsed)
	}
}
