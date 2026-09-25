package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/analytics"
	"github.com/Prajwal-Prathiksh/battery-zen/internal/config"
	"github.com/Prajwal-Prathiksh/battery-zen/internal/lifecycle"
	"github.com/Prajwal-Prathiksh/battery-zen/internal/lock"
	"github.com/Prajwal-Prathiksh/battery-zen/internal/logfile"
	"github.com/Prajwal-Prathiksh/battery-zen/internal/power"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) == 1 {
		tuiCmd()
		return
	}

	switch os.Args[1] {
	case "sample":
		sampleCmd()
	case "run":
		runCmd()
	case "trim":
		trimCmd()
	case "status":
		statusCmd()
	case "tui":
		tuiCmd()
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `battery-zen commands:
  sample     Append one CSV sample (used by systemd timer)
  run        Daemon loop (periodic)
  trim       Force trim to max_lines
  status     Print current reading and path
  tui        Launch interactive TUI for data visualization
`)
	os.Exit(2)
}

func resolvePaths() (config.Config, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, "", err
	}
	logPath, err := config.LogPath(cfg)
	if err != nil {
		return cfg, "", err
	}
	if err := logfile.EnsureDir(logPath); err != nil {
		return cfg, "", err
	}
	return cfg, logPath, nil
}

func loadPaths() (config.Config, string) {
	cfg, logPath, err := resolvePaths()
	if err != nil {
		log.Fatalf("paths: %v", err)
	}
	return cfg, logPath
}

func sampleOnce(cfg config.Config, logPath string, provider power.Provider) (power.Reading, error) {
	reading, err := provider.Read()
	if err != nil {
		return power.Reading{}, err
	}
	writer := &logfile.Writer{Path: logPath}
	timestamp := config.Now(cfg).Format(time.RFC3339)
	if err := writer.Append(timestamp, reading); err != nil {
		return reading, err
	}
	// Trim if we exceeded threshold
	lines, err := writer.LineCount()
	if err == nil && lines > (cfg.MaxLines+cfg.TrimBuffer+1) { // +1 header
		if err := writer.TrimToLast(cfg.MaxLines); err != nil {
			return reading, err
		}
	}
	return reading, nil
}

func sampleCmd() {
	cfg, logPath := loadPaths()
	if _, err := sampleOnce(cfg, logPath, power.New()); err != nil {
		log.Fatalf("sample: %v", err)
	}
}

func runCmd() {
	cfg, logPath := loadPaths()
	closeLog, err := configureRunLogging(cfg)
	if err != nil {
		log.Fatalf("logging: %v", err)
	}
	defer closeLog()
	// Guard with a platform lock so only one daemon runs
	instance := lock.New(filepath.Join(cfg.LogDir, ".battery-zen.lock"))
	ok, err := instance.Acquire()
	if err != nil {
		log.Fatalf("lock: %v", err)
	}
	if !ok {
		return
	}
	defer instance.Release()
	bootID, err := lifecycle.BootID()
	if err != nil {
		log.Printf("boot identity: %v", err)
	}
	monitor, err := lifecycle.Start()
	if err != nil {
		log.Printf("lifecycle monitor: %v", err)
	}
	if monitor != nil {
		defer monitor.Close()
	}
	runSamplingLoop(cfg, logPath, power.New(), nil, monitor, bootID)
}

func runSamplingLoop(cfg config.Config, logPath string, provider power.Provider, stop <-chan struct{}, monitor lifecycle.Monitor, bootID string) {
	store := &lifecycle.Store{Path: filepath.Join(cfg.LogDir, "events.csv")}
	reading, err := sampleOnce(cfg, logPath, provider)
	if err != nil {
		log.Printf("sample: %v", err)
		reading = power.Reading{Percent: -1}
	}
	startupTime := config.Now(cfg)
	existingRecords, err := lifecycle.Read(store.Path)
	if err != nil {
		log.Printf("lifecycle history: %v", err)
	}
	previousBootID := ""
	for i := len(existingRecords) - 1; i >= 0; i-- {
		if existingRecords[i].BootID != "" {
			previousBootID = existingRecords[i].BootID
			break
		}
	}
	if lifecycle.BootChanged(previousBootID, bootID) {
		if err := store.Append(lifecycle.Event{Kind: lifecycle.Restart, Time: startupTime}, bootID, reading); err != nil {
			log.Printf("lifecycle restart: %v", err)
		}
	}
	if err := store.Append(lifecycle.Event{Kind: lifecycle.Startup, Time: startupTime}, bootID, reading); err != nil {
		log.Printf("lifecycle startup: %v", err)
	}
	var events <-chan lifecycle.Event
	if monitor != nil {
		events = monitor.Events()
	}
	timer := time.NewTimer(sampleInterval(cfg, reading))
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case event := <-events:
			eventReading, readErr := provider.Read()
			if readErr != nil {
				log.Printf("lifecycle battery reading: %v", readErr)
				eventReading = reading
			} else {
				reading = eventReading
			}
			if strings.EqualFold(cfg.Timezone, "UTC") {
				event.Time = event.Time.UTC()
			}
			if err := store.Append(event, bootID, eventReading); err != nil {
				log.Printf("lifecycle %s: %v", event.Kind, err)
			}
			if event.Ack != nil {
				close(event.Ack)
			}
		case <-timer.C:
			reading, err = sampleOnce(cfg, logPath, provider)
			if err != nil {
				log.Printf("sample: %v", err)
			}
			timer.Reset(sampleInterval(cfg, reading))
		}
	}
}

func sampleInterval(cfg config.Config, reading power.Reading) time.Duration {
	seconds := cfg.IntervalSecs
	if reading.ACConnected && cfg.IntervalSecsOnAC > 0 {
		seconds = cfg.IntervalSecsOnAC
	}
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func trimCmd() {
	cfg, logPath := loadPaths()
	w := &logfile.Writer{Path: logPath}
	if err := w.TrimToLast(cfg.MaxLines); err != nil {
		log.Fatalf("trim: %v", err)
	}
}

func statusCmd() {
	cfg, logPath := loadPaths()
	reading, err := power.New().Read()
	if err != nil {
		log.Fatalf("status: %v", err)
	}
	fmt.Printf("ac_connected=%t battery_life=%d state=%s remaining_capacity_mwh=%s full_charged_capacity_mwh=%s design_capacity_mwh=%s rate_mw=%s voltage_mv=%s cycle_count=%s ts=%s file=%s\n",
		reading.ACConnected,
		reading.Percent,
		reading.State,
		optionalUint32(reading.RemainingCapacityMWh),
		optionalUint32(reading.FullChargedCapacityMWh),
		optionalUint32(reading.DesignCapacityMWh),
		optionalInt32(reading.RateMW),
		optionalUint32(reading.VoltageMV),
		optionalUint32(reading.CycleCount),
		config.Now(cfg).Format(time.RFC3339),
		logPath,
	)
}

func optionalUint32(value *uint32) string {
	if value == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*value), 10)
}

func optionalInt32(value *int32) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(int64(*value), 10)
}

// readCSV reads the battery CSV file and parses it into Row structs
func readCSV(path string) ([]analytics.Row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	return analytics.ParseCSVRows(rows)
}

// findLastACTransition finds the most recent AC status change and returns
// the time and battery percentage when the current AC status started.
// Returns zero time and 0.0 battery if no transition found.
func findLastACTransition(rows []analytics.Row) (time.Time, float64) {
	if len(rows) == 0 {
		return time.Time{}, 0.0
	}

	currentACStatus := rows[len(rows)-1].AC

	// Walk backwards from the end to find the last status change
	for i := len(rows) - 2; i >= 0; i-- {
		if rows[i].AC != currentACStatus {
			// Found the transition point - return the time and battery of the first sample with current status
			if i+1 < len(rows) {
				return rows[i+1].T, rows[i+1].Batt
			}
		}
	}

	// No transition found in the data, current status has been the same throughout
	// Return the time and battery of the first sample
	return rows[0].T, rows[0].Batt
}

// BinDataPoint represents a time bin with battery data
type BinDataPoint struct {
	Time    time.Time
	Batt    float64
	AC      bool
	HasData bool
}

// binDataToTimeGrid bins the data into time intervals for plotting
func binDataToTimeGrid(rows []analytics.Row, binSize time.Duration, window time.Duration) []BinDataPoint {
	if len(rows) == 0 {
		return nil
	}

	// Calculate the time range
	endTime := rows[len(rows)-1].T
	startTime := endTime.Add(-window)

	// Create bins
	var bins []BinDataPoint
	for t := startTime; t.Before(endTime) || t.Equal(endTime); t = t.Add(binSize) {
		bins = append(bins, BinDataPoint{
			Time:    t,
			HasData: false,
		})
	}

	// Fill bins with data
	for _, row := range rows {
		if row.T.Before(startTime) {
			continue
		}

		// Find the appropriate bin
		binIndex := int(row.T.Sub(startTime) / binSize)
		if binIndex >= 0 && binIndex < len(bins) {
			bins[binIndex].Batt = row.Batt
			bins[binIndex].AC = row.AC
			bins[binIndex].HasData = true
		}
	}

	return bins
}

// createTimeBasedSeries creates series data with intelligent time-based x-labels for zooming
func createTimeBasedSeries(bins []BinDataPoint, startTime time.Time) ([]float64, []float64, map[int]string) {
	acSeries := make([]float64, len(bins))
	battSeries := make([]float64, len(bins))

	// Create intelligent time labels based on data density
	labels := make(map[int]string)

	for i, bin := range bins {
		if bin.HasData {
			if bin.AC {
				acSeries[i] = bin.Batt
				battSeries[i] = math.NaN()
			} else {
				battSeries[i] = bin.Batt
				acSeries[i] = math.NaN()
			}
		} else {
			acSeries[i] = math.NaN()
			battSeries[i] = math.NaN()
		}
	}

	// Create time labels for all points - termdash will intelligently display them
	// We'll provide labels at different granularities for different zoom levels
	for i, bin := range bins {
		minute := bin.Time.Minute()

		// Always provide a time label - format depends on the time
		if minute == 0 {
			// Top of the hour - show HH:00
			labels[i] = bin.Time.Format("15:04")
		} else if minute%15 == 0 {
			// Quarter hours - show HH:15, HH:30, HH:45
			labels[i] = bin.Time.Format("15:04")
		} else if minute%5 == 0 {
			// Every 5 minutes - useful for zoomed views
			labels[i] = bin.Time.Format("15:04")
		} else {
			// For very zoomed views, show all times
			labels[i] = bin.Time.Format("15:04")
		}
	}

	return acSeries, battSeries, labels
}

// tuiCmd implements the TUI command using termdash with real-time parameter controls
func tuiCmd() {
	runTUI()
}
