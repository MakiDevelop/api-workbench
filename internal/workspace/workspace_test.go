package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInfoIncludesRequestMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".apiw", "env"), 0o755); err != nil {
		t.Fatalf("mkdir env: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "requests", "users"), 0o755); err != nil {
		t.Fatalf("mkdir requests: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".apiw", "apiw.json"), []byte(`{"schemaVersion":1}`), 0o644); err != nil {
		t.Fatalf("write apiw.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".apiw", "env", "local.env"), []byte("BASE_URL=https://example.com\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "requests", "users", "get-profile.json"), []byte(`{
  "name": "Get Profile",
  "method": "get",
  "url": "${BASE_URL}/users/me",
  "headers": {
    "Authorization": "Bearer ${TOKEN}"
  },
  "query": {
    "include": "teams"
  },
  "body": {
    "type": "json",
    "content": {
      "preview": true
    }
  },
  "assertions": [
    {"type": "status", "equals": 200},
    {"type": "body_contains", "contains": "profile"}
  ]
}`), 0o644); err != nil {
		t.Fatalf("write request: %v", err)
	}

	info, err := LoadInfo(root, "")
	if err != nil {
		t.Fatalf("load info: %v", err)
	}

	if info.CollectionPath != "requests" {
		t.Fatalf("expected collection path requests, got %s", info.CollectionPath)
	}
	if len(info.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(info.Requests))
	}

	entry := info.Requests[0]
	if entry.Name != "Get Profile" {
		t.Fatalf("unexpected request name: %s", entry.Name)
	}
	if entry.Path != filepath.ToSlash(filepath.Join("requests", "users", "get-profile.json")) {
		t.Fatalf("unexpected request path: %s", entry.Path)
	}
	if entry.Method != "GET" {
		t.Fatalf("expected uppercase method, got %s", entry.Method)
	}
	if entry.URL != "${BASE_URL}/users/me" {
		t.Fatalf("unexpected url: %s", entry.URL)
	}
	if entry.Headers["Authorization"] != "Bearer ${TOKEN}" {
		t.Fatalf("unexpected auth header: %s", entry.Headers["Authorization"])
	}
	if entry.Query["include"] != "teams" {
		t.Fatalf("unexpected include query: %s", entry.Query["include"])
	}
	if entry.Body == nil {
		t.Fatal("expected body preview")
	}
	if entry.Body.Type != "json" {
		t.Fatalf("unexpected body type: %s", entry.Body.Type)
	}
	if entry.Body.Content == "" {
		t.Fatal("expected pretty-printed body content")
	}
	if len(entry.Assertions) != 2 {
		t.Fatalf("expected assertions, got %d", len(entry.Assertions))
	}
}
