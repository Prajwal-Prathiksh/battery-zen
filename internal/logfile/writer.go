package logfile

import (
	"bufio"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Prajwal-Prathiksh/battery-zen/internal/power"
)

type Writer struct {
	Path string
}

var columns = []string{
	"timestamp",
	"ac_connected",
	"battery_life",
	"state",
	"remaining_capacity_mwh",
	"full_charged_capacity_mwh",
	"design_capacity_mwh",
	"rate_mw",
	"voltage_mv",
	"cycle_count",
}

// Append a CSV row (write header if file didn't exist)
func (w *Writer) Append(timestamp string, reading power.Reading) error {
	info, err := os.Stat(w.Path)
	newFile := errors.Is(err, os.ErrNotExist) || err == nil && info.Size() == 0
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(w.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := bufio.NewWriter(file)
	writer := csv.NewWriter(buffer)
	if newFile {
		if err := writer.Write(columns); err != nil {
			return err
		}
	}
	if err := writer.Write(sampleRecord(timestamp, reading)); err != nil {
		return err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	return buffer.Flush()
}

func sampleRecord(timestamp string, reading power.Reading) []string {
	ac := "0"
	if reading.ACConnected {
		ac = "1"
	}
	return []string{
		timestamp,
		ac,
		strconv.Itoa(reading.Percent),
		string(reading.State),
		formatUint32(reading.RemainingCapacityMWh),
		formatUint32(reading.FullChargedCapacityMWh),
		formatUint32(reading.DesignCapacityMWh),
		formatInt32(reading.RateMW),
		formatUint32(reading.VoltageMV),
		formatUint32(reading.CycleCount),
	}
}

func formatUint32(value *uint32) string {
	if value == nil {
		return ""
	}
	return strconv.FormatUint(uint64(*value), 10)
}

func formatInt32(value *int32) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(int64(*value), 10)
}

// Count lines quickly enough for ~1k lines
func (w *Writer) LineCount() (int, error) {
	f, err := os.Open(w.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	count := 0
	for {
		_, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// Keep header + last N data lines (atomic replace)
func (w *Writer) TrimToLast(maxDataLines int) error {
	// Read existing file; if not found, nothing to do
	src, err := os.Open(w.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer src.Close()

	// Read header
	br := bufio.NewReader(src)
	header, err := br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	// Tail last N data lines by reading file backwards
	dataLines, err := tailLastLines(src, maxDataLines)
	if err != nil {
		return err
	}
	if err := src.Close(); err != nil {
		return err
	}

	tmp := w.Path + ".tmp"
	dst, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()

	bw := bufio.NewWriter(dst)
	if header != "" {
		if _, err := bw.WriteString(header); err != nil {
			return err
		}
	}
	for i := range dataLines {
		if _, err := bw.WriteString(dataLines[i]); err != nil {
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, w.Path) // atomic within same dir
}

// tailLastLines reads the last N lines (excluding header) efficiently.
func tailLastLines(f *os.File, n int) ([]string, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size == 0 {
		return nil, nil
	}

	lines, err := readLinesBackward(f, size, n)
	if err != nil {
		return nil, err
	}

	// Reverse to chronological order
	reverseStrings(lines)

	// Ensure trailing newline on each
	for i := range lines {
		if !strings.HasSuffix(lines[i], "\n") {
			lines[i] += "\n"
		}
	}

	return lines, nil
}

func readLinesBackward(f *os.File, size int64, n int) ([]string, error) {
	const chunk = 8192
	var (
		buf   []byte
		pos   = size
		lines []string
	)

	partial := []byte{}
	for pos > 0 && len(lines) <= n {
		readSize := pos
		if readSize > chunk {
			readSize = chunk
		}
		pos -= readSize

		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			return nil, err
		}

		tmp := make([]byte, readSize)
		if _, err := io.ReadFull(f, tmp); err != nil {
			return nil, err
		}

		buf = append(tmp, partial...)
		lines, buf = extractLinesFromEnd(buf, lines, n)
		partial = buf
		buf = buf[:0]
	}

	// Handle remaining partial line
	if len(lines) < n && len(partial) > 0 {
		lines = append(lines, string(partial))
	}

	return lines, nil
}

func extractLinesFromEnd(buf []byte, lines []string, n int) ([]string, []byte) {
	for i := len(buf) - 1; i >= 0 && len(lines) < n; i-- {
		if buf[i] == '\n' {
			line := string(buf[i+1:])
			if line != "" {
				lines = append(lines, line)
			}
			buf = buf[:i]
		}
	}
	return lines, buf
}

func reverseStrings(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
