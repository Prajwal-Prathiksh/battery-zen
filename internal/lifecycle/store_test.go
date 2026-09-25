package lifecycle

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/power"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.csv")
	store := &Store{Path: path}
	timestamp := time.Date(2026, 1, 1, 12, 0, 0, 123, time.UTC)
	reading := power.Reading{ACConnected: false, Percent: 64}
	if err := store.Append(Event{Kind: Suspend, Time: timestamp}, "boot-a", reading); err != nil {
		t.Fatal(err)
	}
	records, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	record := records[0]
	if !record.Time.Equal(timestamp) || record.Kind != Suspend || record.BootID != "boot-a" || record.ACConnected || record.Battery != 64 {
		t.Fatalf("unexpected record: %+v", record)
	}
}
