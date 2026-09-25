package logfile

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/power"
)

func TestAppendExtendedSample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.csv")
	remaining := uint32(40_000)
	full := uint32(80_000)
	design := uint32(90_000)
	rate := int32(-12_500)
	voltage := uint32(11_500)
	cycles := uint32(42)
	reading := power.Reading{
		ACConnected:            false,
		Percent:                50,
		State:                  power.StateDischarging,
		RemainingCapacityMWh:   &remaining,
		FullChargedCapacityMWh: &full,
		DesignCapacityMWh:      &design,
		RateMW:                 &rate,
		VoltageMV:              &voltage,
		CycleCount:             &cycles,
	}
	if err := (&Writer{Path: path}).Append("2026-01-01T00:00:00Z", reading); err != nil {
		t.Fatal(err)
	}
	rows := readTestCSV(t, path)
	if len(rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows))
	}
	if len(rows[0]) != len(columns) || len(rows[1]) != len(columns) {
		t.Fatalf("column counts = %d and %d, want %d", len(rows[0]), len(rows[1]), len(columns))
	}
	if rows[1][6] != "90000" || rows[1][7] != "-12500" || rows[1][9] != "42" {
		t.Fatalf("unexpected extended row: %v", rows[1])
	}
}

func TestTrimToLast(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs.csv")
	writer := &Writer{Path: path}
	for percent := 0; percent < 5; percent++ {
		if err := writer.Append("2026-01-01T00:00:00Z", power.Reading{Percent: percent}); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.TrimToLast(2); err != nil {
		t.Fatal(err)
	}
	rows := readTestCSV(t, path)
	if len(rows) != 3 || rows[1][2] != "3" || rows[2][2] != "4" {
		t.Fatalf("unexpected trimmed rows: %v", rows)
	}
}

func readTestCSV(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
