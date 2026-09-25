package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/analytics"
	"github.com/Prajwal-Prathiksh/battery-zen/internal/config"
)

func TestGenerateStatusInfoFullRangeProjection(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		ac        bool
		startBatt float64
		endBatt   float64
		label     string
	}{
		{"discharging", false, 80, 70, "Projected full-battery runtime (100% → 0%)"},
		{"charging", true, 20, 30, "Projected full charge time (0% → 100%)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := []analytics.Row{
				{T: now.Add(-10 * time.Minute), AC: test.ac, Batt: test.startBatt},
				{T: now, AC: test.ac, Batt: test.endBatt},
			}
			status := GenerateStatusInfo(rows, nil, 0.05, &UIParams{}, "telemetry.csv", config.Defaults())
			if status.FullRangeLabel != test.label {
				t.Fatalf("label = %q, want %q", status.FullRangeLabel, test.label)
			}
			if status.FullRangeEstimate != "01h 40m" {
				t.Fatalf("full-range estimate = %q, want 01h 40m", status.FullRangeEstimate)
			}
			found := false
			for _, line := range BuildStatusLines(status) {
				if strings.Contains(line.Text, test.label+": 01h 40m") {
					found = true
					break
				}
			}
			if !found {
				t.Fatal("full-range projection is missing from status display")
			}
		})
	}
}

func TestGenerateStatusInfoRejectsInfiniteEstimate(t *testing.T) {
	now := time.Now()
	rows := []analytics.Row{
		{T: now.Add(-time.Minute), AC: true, Batt: 30},
		{T: now, AC: true, Batt: 30},
	}
	status := GenerateStatusInfo(rows, nil, 0.05, &UIParams{}, "telemetry.csv", config.Defaults())
	if status.Estimate != "—" {
		t.Fatalf("estimate = %q, want —", status.Estimate)
	}
	if status.EstimateDuration != 0 {
		t.Fatalf("duration = %v, want zero", status.EstimateDuration)
	}
	if status.FullRangeEstimate != "—" {
		t.Fatalf("full-range estimate = %q, want —", status.FullRangeEstimate)
	}
	if status.EventLogPath != "events.csv" {
		t.Fatalf("event log path = %q, want events.csv", status.EventLogPath)
	}
	found := false
	for _, line := range BuildStatusLines(status) {
		if strings.Contains(line.Text, "Events file: events.csv") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("events path is missing from status display")
	}
}
