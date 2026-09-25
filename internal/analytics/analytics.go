package analytics

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/lifecycle"
)

// Row represents a single CSV record
type Row struct {
	T    time.Time
	AC   bool
	Batt float64
}

// ParseBoolLoose parses boolean values in various formats including
// "true"/"false", "t"/"f", "1"/"0", "yes"/"no", "y"/"n", and integers
// where 0 is false and any other value is true.
func ParseBoolLoose(s string) (bool, error) {
	ss := strings.TrimSpace(strings.ToLower(s))
	switch ss {
	case "true", "t", "1", "yes", "y":
		return true, nil
	case "false", "f", "0", "no", "n":
		return false, nil
	default:
		// Try parsing as integer (0 = false, anything else = true)
		if val, err := strconv.Atoi(ss); err == nil {
			return val != 0, nil
		}
		return false, fmt.Errorf("bad bool: %q", s)
	}
}

// WeightedLinReg performs weighted linear regression on battery data
// using exponential weights (more recent data has higher weight).
// x represents minutes relative to the last point (<=0), weights w = exp(alpha*x).
// Returns slope b (% per minute), intercept a (% at x=0 i.e., "now"), and success flag.
func WeightedLinReg(rows []Row, alpha float64) (float64, float64, bool) {
	if len(rows) < 2 {
		return 0, 0, false
	}
	tNow := rows[len(rows)-1].T

	var sumW, sumWX, sumWY, sumWXX, sumWXY float64
	for _, r := range rows {
		x := r.T.Sub(tNow).Minutes() // <= 0
		w := math.Exp(alpha * x)     // more recent -> larger weight
		y := r.Batt
		sumW += w
		sumWX += w * x
		sumWY += w * y
		sumWXX += w * x * x
		sumWXY += w * x * y
	}

	den := sumW*sumWXX - sumWX*sumWX
	if den == 0 {
		return 0, 0, false
	}
	b := (sumW*sumWXY - sumWX*sumWY) / den
	a := (sumWY - b*sumWX) / sumW
	return b, a, true
}

// FmtDur formats a duration in minutes into a human-readable string
// in the format "Xh Ym". Returns "—" for invalid values (NaN, Inf, or negative).
func FmtDur(mins float64) string {
	if math.IsNaN(mins) || math.IsInf(mins, 0) || mins < 0 {
		return "—"
	}
	d := time.Duration(mins * float64(time.Minute))
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}

// FilterContiguousACState filters rows to include only the most recent contiguous
// samples with the specified AC state (true for plugged, false for unplugged).
// Returns rows in chronological order (oldest first).
func FilterContiguousACState(rows []Row, acState bool) []Row {
	if len(rows) == 0 {
		return nil
	}

	var filtered []Row
	// Walk backwards from the end to find contiguous samples with the specified AC state
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].AC == acState {
			filtered = append([]Row{rows[i]}, filtered...)
		} else {
			break
		}
	}
	return filtered
}

// CalculateRateAndEstimate calculates the battery rate and time estimate based on AC state.
// For charging (AC=true): returns positive rate and time to maxChargePercent.
// For discharging (AC=false): returns negative rate and time to 0%.
// Returns rate (% per minute), estimate (minutes), confidence string, and success flag.
func CalculateRateAndEstimate(rows []Row, currentBatt float64, alpha float64, maxChargePercent int) (float64, float64, string, bool) {
	if len(rows) < 2 {
		return 0, 0, "(need ≥2 samples with same AC state)", false
	}

	// Determine if we're looking at charging or discharging data
	isCharging := rows[0].AC // All rows should have the same AC state due to filtering

	rate, _, ok := WeightedLinReg(rows, alpha)
	if !ok {
		return 0, 0, "(regression failed)", false
	}

	var estimate float64
	var confidence string

	if isCharging {
		// When charging, rate should be positive (battery % increasing)
		if rate > 1e-6 { // Positive rate means charging
			estimate = (float64(maxChargePercent) - currentBatt) / rate // Time to reach max charge
			confidence = fmt.Sprintf("(based on %d charging samples)", len(rows))
		} else {
			// Rate is negative or zero while plugged in - not actually charging
			estimate = math.Inf(1) // Infinite time (already at max or discharging while plugged)
			confidence = "(not charging or already full)"
		}
	} else {
		// When discharging, rate should be negative (battery % decreasing)
		if rate < -1e-6 { // Negative rate means discharging
			estimate = -currentBatt / rate // Time to reach 0%
			confidence = fmt.Sprintf("(based on %d discharging samples)", len(rows))
		} else {
			// Rate is positive or zero while unplugged - unusual
			estimate = math.Inf(1) // Infinite time (not actually discharging)
			confidence = "(not discharging)"
		}
	}

	return rate, estimate, confidence, true
}

// ParseCSVRows parses CSV data with flexible column detection and
// converts it to a slice of Row structs. The CSV must contain
// timestamp, AC connection status, and battery percentage columns.
// Column names are matched case-insensitively with various aliases supported.
func ParseCSVRows(rows [][]string) ([]Row, error) {
	if len(rows) == 0 {
		return nil, errors.New("empty csv")
	}

	tsIdx, acIdx, battIdx, err := findColumns(rows[0])
	if err != nil {
		return nil, err
	}

	var out []Row
	for i := 1; i < len(rows); i++ {
		row, err := parseCSVRow(rows[i], tsIdx, acIdx, battIdx)
		if err != nil {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func findColumns(header []string) (tsIdx, acIdx, battIdx int, err error) {
	col := func(name string) int {
		name = strings.ToLower(strings.TrimSpace(name))
		for i, h := range header {
			if strings.ToLower(strings.TrimSpace(h)) == name {
				return i
			}
		}
		return -1
	}

	tsIdx = col("timestamp")
	acIdx = col("ac_connected")
	if acIdx == -1 {
		acIdx = col("ac")
	}
	if acIdx == -1 {
		acIdx = col("ac plugged in (bool)")
	}
	if acIdx == -1 {
		acIdx = col("ac plugged in")
	}
	battIdx = col("battery_life")
	if battIdx == -1 {
		battIdx = col("battery")
	}
	if battIdx == -1 {
		battIdx = col("battery life (%)")
	}

	if tsIdx == -1 || acIdx == -1 || battIdx == -1 {
		return -1, -1, -1, fmt.Errorf("expected headers: timestamp, ac_connected, battery_life (or similar)")
	}
	return tsIdx, acIdx, battIdx, nil
}

func parseCSVRow(rec []string, tsIdx, acIdx, battIdx int) (Row, error) {
	if len(rec) <= battIdx || len(rec) <= tsIdx || len(rec) <= acIdx {
		return Row{}, fmt.Errorf("insufficient columns")
	}

	t, err := parseTimestamp(strings.TrimSpace(rec[tsIdx]))
	if err != nil {
		return Row{}, err
	}

	ac, err := ParseBoolLoose(rec[acIdx])
	if err != nil {
		return Row{}, err
	}

	b, err := strconv.ParseFloat(strings.TrimSpace(rec[battIdx]), 64)
	if err != nil {
		return Row{}, err
	}

	return Row{T: t, AC: ac, Batt: b}, nil
}

func parseTimestamp(tsStr string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, tsStr)
	if err == nil {
		return t, nil
	}

	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02T15:04:05",
	}
	for _, lay := range layouts {
		if tt, e2 := time.Parse(lay, tsStr); e2 == nil {
			return tt, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse timestamp")
}

// SuspendEvent represents an observed suspend, shutdown, logoff, or restart period
type SuspendEvent struct {
	Kind          lifecycle.Kind
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	BatteryBefore float64
	BatteryAfter  float64
	BatteryDrop   float64
}

// DetectSuspendEvents pairs explicit lifecycle transitions into inactive periods.
func DetectSuspendEvents(rows []Row, records []lifecycle.Record) []SuspendEvent {
	ordered := append([]lifecycle.Record(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Time.Before(ordered[j].Time) })
	var events []SuspendEvent
	var inactive *lifecycle.Record
	var lastBootID string
	for i := range ordered {
		record := ordered[i]
		switch record.Kind {
		case lifecycle.Suspend, lifecycle.Shutdown, lifecycle.Logoff:
			if inactive == nil {
				copy := record
				inactive = &copy
			} else if record.Kind == lifecycle.Shutdown {
				inactive.Kind = lifecycle.Shutdown
			}
		case lifecycle.Resume:
			if inactive != nil {
				events = append(events, lifecyclePeriod(*inactive, record))
				inactive = nil
			}
		case lifecycle.Startup, lifecycle.Restart:
			if inactive != nil {
				events = append(events, lifecyclePeriod(*inactive, record))
				inactive = nil
			} else if lifecycle.BootChanged(lastBootID, record.BootID) {
				if previous, ok := latestRowBefore(rows, record.Time); ok {
					events = append(events, SuspendEvent{
						Kind:          lifecycle.Restart,
						StartTime:     previous.T,
						EndTime:       record.Time,
						Duration:      record.Time.Sub(previous.T),
						BatteryBefore: previous.Batt,
						BatteryAfter:  record.Battery,
						BatteryDrop:   previous.Batt - record.Battery,
					})
				}
			}
			if record.BootID != "" {
				lastBootID = record.BootID
			}
		}
	}
	return events
}

func lifecyclePeriod(start, end lifecycle.Record) SuspendEvent {
	return SuspendEvent{
		Kind:          start.Kind,
		StartTime:     start.Time,
		EndTime:       end.Time,
		Duration:      end.Time.Sub(start.Time),
		BatteryBefore: start.Battery,
		BatteryAfter:  end.Battery,
		BatteryDrop:   start.Battery - end.Battery,
	}
}

func latestRowBefore(rows []Row, timestamp time.Time) (Row, bool) {
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].T.Before(timestamp) {
			return rows[i], true
		}
	}
	return Row{}, false
}

// ScreenOnTimeResult holds screen-on time calculation results
type ScreenOnTimeResult struct {
	TotalActiveTime   time.Duration  // Total time with active data points
	SuspendTime       time.Duration  // Total time in suspend/shutdown
	LastActiveSession time.Duration  // Active time since last suspend/wake
	SuspendEvents     []SuspendEvent // All suspend events in the period
}

// CalculateScreenOnTime calculates active time from explicit lifecycle events.
func CalculateScreenOnTime(rows []Row, records []lifecycle.Record) ScreenOnTimeResult {
	if len(rows) < 2 {
		return ScreenOnTimeResult{}
	}
	return calculateScreenOnTimeRange(rows, records, rows[0].T, rows[len(rows)-1].T)
}

// CalculateDailyScreenOnTime calculates screen-on time for a specific day.
func CalculateDailyScreenOnTime(rows []Row, records []lifecycle.Record, targetDate time.Time) ScreenOnTimeResult {
	if len(rows) < 2 {
		return ScreenOnTimeResult{}
	}
	startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)
	start := startOfDay
	if rows[0].T.After(start) {
		start = rows[0].T
	}
	end := endOfDay
	if rows[len(rows)-1].T.Before(end) {
		end = rows[len(rows)-1].T
	}
	if !end.After(start) {
		return ScreenOnTimeResult{}
	}
	return calculateScreenOnTimeRange(rows, records, start, end)
}

func calculateScreenOnTimeRange(rows []Row, records []lifecycle.Record, start, end time.Time) ScreenOnTimeResult {
	result := ScreenOnTimeResult{SuspendEvents: DetectSuspendEvents(rows, records)}
	for _, event := range result.SuspendEvents {
		clippedStart := event.StartTime
		if clippedStart.Before(start) {
			clippedStart = start
		}
		clippedEnd := event.EndTime
		if clippedEnd.After(end) {
			clippedEnd = end
		}
		if clippedEnd.After(clippedStart) {
			result.SuspendTime += clippedEnd.Sub(clippedStart)
		}
	}
	result.TotalActiveTime = end.Sub(start) - result.SuspendTime
	activeStart := start
	inactive := false
	lastBootID := ""
	ordered := append([]lifecycle.Record(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Time.Before(ordered[j].Time) })
	for _, record := range ordered {
		if record.Time.After(end) {
			break
		}
		beforeRange := record.Time.Before(start)
		switch record.Kind {
		case lifecycle.Resume:
			if !beforeRange {
				activeStart = record.Time
			}
			inactive = false
		case lifecycle.Startup, lifecycle.Restart:
			bootChanged := lifecycle.BootChanged(lastBootID, record.BootID)
			if !beforeRange && (inactive || bootChanged || lastBootID == "") {
				activeStart = record.Time
			}
			inactive = false
			if record.BootID != "" {
				lastBootID = record.BootID
			}
		case lifecycle.Suspend, lifecycle.Shutdown, lifecycle.Logoff:
			inactive = true
		}
	}
	if !inactive && end.After(activeStart) {
		result.LastActiveSession = end.Sub(activeStart)
	}
	return result
}
