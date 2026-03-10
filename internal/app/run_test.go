package app

import (
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
