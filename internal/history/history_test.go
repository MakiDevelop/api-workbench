package history

import (
	"testing"
	"time"
)

func TestAppendAndList(t *testing.T) {
	root := t.TempDir()

	entries := []Entry{
		{Timestamp: "2026-04-10T10:00:00Z", RequestName: "list-users", Method: "GET", URL: "https://api/users", StatusCode: 200, ExitCode: 0},
		{Timestamp: "2026-04-10T10:01:00Z", RequestName: "create-user", Method: "POST", URL: "https://api/users", StatusCode: 201, ExitCode: 0},
		{Timestamp: "2026-04-10T10:02:00Z", RequestName: "bad-request", Method: "GET", URL: "https://api/bad", StatusCode: 500, ExitCode: 3, Error: "assertion failed"},
	}

	for _, e := range entries {
		if err := Append(root, e); err != nil {
			t.Fatal(err)
		}
	}

	listed, err := List(root, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(listed) != 3 {
		t.Fatalf("got %d entries, want 3", len(listed))
	}

	// Should be newest first (reverse insertion order).
	if listed[0].RequestName != "bad-request" {
		t.Errorf("newest entry = %q, want bad-request", listed[0].RequestName)
	}
	if listed[2].RequestName != "list-users" {
		t.Errorf("oldest entry = %q, want list-users", listed[2].RequestName)
	}
}

func TestListLimit(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 10; i++ {
		Append(root, Entry{
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
			RequestName: "test",
			Method:      "GET",
		})
	}

	listed, err := List(root, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 3 {
		t.Errorf("limit=3, got %d entries", len(listed))
	}
}

func TestPrune(t *testing.T) {
	root := t.TempDir()

	// Append an entry from 60 days ago.
	old := time.Now().UTC().AddDate(0, 0, -60).Format(time.RFC3339)
	Append(root, Entry{Timestamp: old, RequestName: "old"})

	// Append a recent entry.
	Append(root, Entry{Timestamp: time.Now().UTC().Format(time.RFC3339), RequestName: "new"})

	// Prune keeping last 30 days.
	if err := Prune(root, 30); err != nil {
		t.Fatal(err)
	}

	listed, err := List(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range listed {
		if e.RequestName == "old" {
			t.Errorf("old entry should have been pruned: %+v", e)
		}
	}
}

func TestEmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	entries, err := List(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %+v", entries)
	}
}
