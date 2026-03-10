package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRequestAcceptsRequestFileBeforeFlags(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".apiw", "env"), 0o755); err != nil {
		t.Fatalf("mkdir env: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "requests"), 0o755); err != nil {
		t.Fatalf("mkdir requests: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".apiw", "apiw.json"), []byte(`{"schemaVersion":1}`), 0o644); err != nil {
		t.Fatalf("write apiw.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".apiw", "env", "local.env"), []byte("BASE_URL=http://127.0.0.1:1\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "requests", "health.json"), []byte(`{
  "name": "health",
  "method": "GET",
  "url": "${BASE_URL}/"
}`), 0o644); err != nil {
		t.Fatalf("write request: %v", err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(previous)
	}()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout strings.Builder
	var stderr strings.Builder

	code, err := runRequest([]string{"requests/health.json", "--env", "local"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRunRequestAllRunsCollection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".apiw", "env"), 0o755); err != nil {
		t.Fatalf("mkdir env: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "requests", "smoke"), 0o755); err != nil {
		t.Fatalf("mkdir requests: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".apiw", "apiw.json"), []byte(`{"schemaVersion":1}`), 0o644); err != nil {
		t.Fatalf("write apiw.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".apiw", "env", "local.env"), []byte("BASE_URL=http://127.0.0.1:1\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".apiw", "env", "staging.env"), []byte("BASE_URL="+server.URL+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "requests", "smoke", "health.json"), []byte(`{
  "name": "health",
  "method": "GET",
  "url": "${BASE_URL}/health",
  "assertions": [{"type":"status","equals":200}]
}`), 0o644); err != nil {
		t.Fatalf("write request: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "requests", "smoke", "version.json"), []byte(`{
  "name": "version",
  "method": "GET",
  "url": "${BASE_URL}/version",
  "assertions": [{"type":"body_contains","contains":"1.0.0"}]
}`), 0o644); err != nil {
		t.Fatalf("write request: %v", err)
	}

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(previous)
	}()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var stdout strings.Builder
	var stderr strings.Builder

	code, err := runRequest([]string{"--all", "requests/smoke", "--env", "staging", "--snapshot"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "summary        total=2 passed=2 failed=0 transport=0 invalid=0") {
		t.Fatalf("missing summary in stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "snapshot       ") {
		t.Fatalf("expected snapshot output, got %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".apiw", "snapshots", "health--staging.json")); err != nil {
		t.Fatalf("expected health snapshot, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".apiw", "snapshots", "version--staging.json")); err != nil {
		t.Fatalf("expected version snapshot, got %v", err)
	}
}
