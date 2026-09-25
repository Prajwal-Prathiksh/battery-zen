package lifecycle

import (
	"encoding/csv"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/power"
)

type Record struct {
	Time        time.Time
	Kind        Kind
	BootID      string
	ACConnected bool
	Battery     float64
}

type Store struct {
	Path string
}

var eventColumns = []string{"timestamp", "event", "boot_id", "ac_connected", "battery_life"}

func (store *Store) Append(event Event, bootID string, reading power.Reading) error {
	info, err := os.Stat(store.Path)
	newFile := errors.Is(err, os.ErrNotExist) || err == nil && info.Size() == 0
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(store.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if newFile {
		if err := writer.Write(eventColumns); err != nil {
			return err
		}
	}
	ac := "0"
	if reading.ACConnected {
		ac = "1"
	}
	if err := writer.Write([]string{
		event.Time.Format(time.RFC3339Nano),
		string(event.Kind),
		bootID,
		ac,
		strconv.Itoa(reading.Percent),
	}); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func Read(path string) ([]Record, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	var records []Record
	for _, row := range rows[1:] {
		if len(row) != len(eventColumns) {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, row[0])
		if err != nil {
			continue
		}
		ac, err := strconv.ParseBool(row[3])
		if err != nil {
			ac = row[3] == "1"
		}
		battery, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			continue
		}
		records = append(records, Record{
			Time:        timestamp,
			Kind:        Kind(row[1]),
			BootID:      row[2],
			ACConnected: ac,
			Battery:     battery,
		})
	}
	return records, nil
}
