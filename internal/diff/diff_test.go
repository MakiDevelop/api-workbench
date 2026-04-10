package diff

import (
	"encoding/json"
	"testing"
)

func TestSnapshotsIdentical(t *testing.T) {
	snap := `{"requestName":"test","environment":"local","capturedAt":"2026-04-10T00:00:00Z","method":"GET","url":"https://example.com","statusCode":200,"headers":{"Content-Type":"application/json"},"body":"{\"ok\":true}","durationMs":42}`

	d, err := Snapshots([]byte(snap), []byte(snap))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Same {
		t.Errorf("expected same=true, got changes: %+v", d.Changes)
	}
}

func TestSnapshotsStatusChange(t *testing.T) {
	left := makeSnap(200, 42, `{"ok":true}`)
	right := makeSnap(500, 100, `{"error":"internal"}`)

	d, err := Snapshots(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if d.Same {
		t.Fatal("expected differences")
	}

	found := map[string]bool{}
	for _, c := range d.Changes {
		found[c.Field] = true
	}
	if !found["statusCode"] {
		t.Error("expected statusCode change")
	}
	if !found["durationMs"] {
		t.Error("expected durationMs change")
	}
	if !found["body"] {
		t.Error("expected body change")
	}
}

func TestSnapshotsHeaderDiff(t *testing.T) {
	left := `{"capturedAt":"2026-01-01T00:00:00Z","statusCode":200,"headers":{"X-Old":"yes","Content-Type":"text/html"},"body":"","durationMs":10}`
	right := `{"capturedAt":"2026-01-02T00:00:00Z","statusCode":200,"headers":{"X-New":"yes","Content-Type":"application/json"},"body":"","durationMs":10}`

	d, err := Snapshots([]byte(left), []byte(right))
	if err != nil {
		t.Fatal(err)
	}

	types := map[string]string{}
	for _, c := range d.Changes {
		types[c.Field] = c.Type
	}

	if types["headers.X-Old"] != "removed" {
		t.Errorf("X-Old should be removed, got %q", types["headers.X-Old"])
	}
	if types["headers.X-New"] != "added" {
		t.Errorf("X-New should be added, got %q", types["headers.X-New"])
	}
	if types["headers.Content-Type"] != "changed" {
		t.Errorf("Content-Type should be changed, got %q", types["headers.Content-Type"])
	}
}

func TestLinesOutput(t *testing.T) {
	d := SnapshotDiff{
		LeftTime:  "2026-01-01",
		RightTime: "2026-01-02",
		Same:      false,
		Changes: []ChangeItem{
			{Field: "statusCode", Type: "changed", Left: "200", Right: "500"},
		},
	}
	text := Lines(d)
	if text == "" {
		t.Error("expected non-empty lines output")
	}
}

func makeSnap(status, duration int, body string) []byte {
	snap := map[string]any{
		"requestName": "test",
		"environment": "local",
		"capturedAt":  "2026-04-10T00:00:00Z",
		"method":      "GET",
		"url":         "https://example.com",
		"statusCode":  status,
		"headers":     map[string]string{"Content-Type": "application/json"},
		"body":        body,
		"durationMs":  duration,
	}
	raw, _ := json.Marshal(snap)
	return raw
}
