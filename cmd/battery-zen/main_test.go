package main

import (
	"testing"
	"time"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/config"
	"github.com/Prajwal-Prathiksh/battery-zen/internal/power"
)

func TestSampleInterval(t *testing.T) {
	cfg := config.Defaults()
	cfg.IntervalSecs = 60
	cfg.IntervalSecsOnAC = 300
	if got := sampleInterval(cfg, power.Reading{ACConnected: false}); got != time.Minute {
		t.Fatalf("battery interval = %v", got)
	}
	if got := sampleInterval(cfg, power.Reading{ACConnected: true}); got != 5*time.Minute {
		t.Fatalf("AC interval = %v", got)
	}
}
