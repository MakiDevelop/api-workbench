package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTUIRunsSelectedRequestThenQuits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

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
	if err := os.WriteFile(filepath.Join(root, ".apiw", "env", "local.env"), []byte("BASE_URL="+server.URL+"\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "requests", "health.json"), []byte(`{
  "name": "health",
  "method": "GET",
  "url": "${BASE_URL}/",
  "assertions": [{"type":"status","equals":200}]
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

	err = runTUI(nil, strings.NewReader("r\nq\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("runTUI returned error: %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "status         passed: requests/health.json (local, snapshot off)") {
		t.Fatalf("missing run status in output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"ok": true`) {
		t.Fatalf("missing rendered body in output: %s", stdout.String())
	}
}

func TestSelectNameOrIndex(t *testing.T) {
	values := []string{"local", "staging", "prod"}

	index, err := selectNameOrIndex(values, "2")
	if err != nil {
		t.Fatalf("selectNameOrIndex returned error: %v", err)
	}
	if index != 1 {
		t.Fatalf("expected index 1, got %d", index)
	}

	index, err = selectNameOrIndex(values, "prod")
	if err != nil {
		t.Fatalf("selectNameOrIndex returned error: %v", err)
	}
	if index != 2 {
		t.Fatalf("expected index 2, got %d", index)
	}
}
