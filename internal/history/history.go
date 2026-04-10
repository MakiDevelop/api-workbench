package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry represents a single API call recorded in history.
type Entry struct {
	Timestamp   string `json:"timestamp"`
	RequestPath string `json:"requestPath"`
	RequestName string `json:"requestName"`
	Env         string `json:"env"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	StatusCode  int    `json:"statusCode"`
	DurationMs  int64  `json:"durationMs"`
	ExitCode    int    `json:"exitCode"`
	Error       string `json:"error,omitempty"`
}

// Append records a single entry to today's history log.
// History files are stored as daily JSONL files at .apiw/history/YYYY-MM-DD.jsonl
func Append(root string, entry Entry) error {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	dir := filepath.Join(root, ".apiw", "history")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Parse timestamp to determine which daily file to write to.
	ts, err := time.Parse(time.RFC3339, entry.Timestamp)
	if err != nil {
		ts = time.Now().UTC()
	}
	day := ts.Format("2006-01-02")
	path := filepath.Join(dir, day+".jsonl")

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// List returns recent history entries across all daily files, newest first.
// limit=0 means unlimited.
func List(root string, limit int) ([]Entry, error) {
	dir := filepath.Join(root, ".apiw", "history")
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Collect .jsonl files and sort descending (newest day first).
	var dayFiles []string
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
			continue
		}
		dayFiles = append(dayFiles, f.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dayFiles)))

	var entries []Entry
	for _, fn := range dayFiles {
		dayEntries, err := readFile(filepath.Join(dir, fn))
		if err != nil {
			continue
		}
		// Reverse day entries so newest first.
		for i := len(dayEntries) - 1; i >= 0; i-- {
			entries = append(entries, dayEntries[i])
			if limit > 0 && len(entries) >= limit {
				return entries, nil
			}
		}
	}

	return entries, nil
}

// Prune removes history files older than keepDays.
func Prune(root string, keepDays int) error {
	dir := filepath.Join(root, ".apiw", "history")
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -keepDays)
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
			continue
		}
		dayStr := strings.TrimSuffix(f.Name(), ".jsonl")
		day, err := time.Parse("2006-01-02", dayStr)
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			os.Remove(filepath.Join(dir, f.Name()))
		}
	}
	return nil
}

func readFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}
