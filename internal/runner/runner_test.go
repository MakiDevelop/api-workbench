package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/request"
)

func TestRunExpandsEnvAndPassesAssertions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "abc123" {
			t.Fatalf("unexpected query token: %s", r.URL.Query().Get("token"))
		}
		w.Header().Set("X-Service", "apiw")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"message":"hello"}`))
	}))
	defer server.Close()

	spec := request.Spec{
		Name:   "health",
		Method: "GET",
		URL:    "${BASE_URL}/check",
		Query: map[string]string{
			"token": "${API_TOKEN}",
		},
		Assertions: []request.Assertion{
			{Type: "status", Equals: 200},
			{Type: "body_contains", Contains: `"ok":true`},
			{Type: "header_equals", Key: "X-Service", Value: "apiw"},
		},
	}

	result, err := Run(spec, Options{
		Variables: map[string]string{
			"BASE_URL":  server.URL,
			"API_TOKEN": "abc123",
		},
		Timeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", result.StatusCode)
	}
	if result.URL != server.URL+"/check?token=abc123" {
		t.Fatalf("unexpected URL: %s", result.URL)
	}
}

func TestRunReturnsAssertionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	spec := request.Spec{
		Name:   "create",
		Method: "POST",
		URL:    server.URL,
		Assertions: []request.Assertion{
			{Type: "status", Equals: 200},
		},
	}

	_, err := Run(spec, Options{Timeout: 3 * time.Second})
	if err == nil {
		t.Fatal("expected assertion error")
	}

	if _, ok := err.(*AssertionError); !ok {
		t.Fatalf("expected AssertionError, got %T", err)
	}
}

func TestWriteSnapshot(t *testing.T) {
	root := t.TempDir()
	spec := request.Spec{Name: "health check"}
	result := Result{
		Method:     "GET",
		URL:        "https://example.com/health",
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"ok":true}`,
		DurationMS: 12,
	}

	path, err := WriteSnapshot(root, "local", spec, result)
	if err != nil {
		t.Fatalf("WriteSnapshot returned error: %v", err)
	}

	if filepath.Base(path) != "health-check--local.json" {
		t.Fatalf("unexpected snapshot path: %s", path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read snapshot: %v", err)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("failed to decode snapshot: %v", err)
	}

	if snapshot.StatusCode != 200 {
		t.Fatalf("unexpected snapshot status: %d", snapshot.StatusCode)
	}
}
