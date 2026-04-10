package curlimport

import (
	"testing"
)

func TestParseSimpleGET(t *testing.T) {
	spec, err := Parse(`curl https://api.example.com/users`)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Method != "GET" {
		t.Errorf("method = %q, want GET", spec.Method)
	}
	if spec.URL != "https://api.example.com/users" {
		t.Errorf("url = %q, want https://api.example.com/users", spec.URL)
	}
	if spec.Name != "users" {
		t.Errorf("name = %q, want users", spec.Name)
	}
}

func TestParsePOSTWithJSON(t *testing.T) {
	spec, err := Parse(`curl -X POST https://api.example.com/users -H "Content-Type: application/json" -d '{"name":"Alice","age":30}'`)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Method != "POST" {
		t.Errorf("method = %q, want POST", spec.Method)
	}
	if spec.Body == nil {
		t.Fatal("body is nil")
	}
	if spec.Body.Type != "json" {
		t.Errorf("body type = %q, want json", spec.Body.Type)
	}
}

func TestParseHeaders(t *testing.T) {
	spec, err := Parse(`curl https://example.com -H "Authorization: Bearer token123" -H "Accept: application/json"`)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Headers["Authorization"] != "Bearer token123" {
		t.Errorf("Authorization = %q", spec.Headers["Authorization"])
	}
	if spec.Headers["Accept"] != "application/json" {
		t.Errorf("Accept = %q", spec.Headers["Accept"])
	}
}

func TestParseQueryParams(t *testing.T) {
	spec, err := Parse(`curl "https://api.example.com/search?q=hello&page=2"`)
	if err != nil {
		t.Fatal(err)
	}
	if spec.URL != "https://api.example.com/search" {
		t.Errorf("url = %q, want query stripped", spec.URL)
	}
	if spec.Query["q"] != "hello" {
		t.Errorf("query q = %q", spec.Query["q"])
	}
	if spec.Query["page"] != "2" {
		t.Errorf("query page = %q", spec.Query["page"])
	}
}

func TestParseFormData(t *testing.T) {
	spec, err := Parse(`curl -X POST https://example.com/upload -F "file=@photo.jpg" -F "name=test"`)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Body == nil || spec.Body.Type != "form" {
		t.Fatalf("body type = %v, want form", spec.Body)
	}
}

func TestParseBasicAuth(t *testing.T) {
	spec, err := Parse(`curl -u admin:secret https://example.com/api`)
	if err != nil {
		t.Fatal(err)
	}
	auth := spec.Headers["Authorization"]
	if auth != "Basic YWRtaW46c2VjcmV0" {
		t.Errorf("Authorization = %q, want Basic YWRtaW46c2VjcmV0", auth)
	}
}

func TestParseLineContinuation(t *testing.T) {
	spec, err := Parse("curl \\\n  -X POST \\\n  https://example.com/api \\\n  -H 'Content-Type: application/json'")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Method != "POST" {
		t.Errorf("method = %q, want POST", spec.Method)
	}
	if spec.URL != "https://example.com/api" {
		t.Errorf("url = %q", spec.URL)
	}
}

func TestParseImpliedPOST(t *testing.T) {
	spec, err := Parse(`curl https://example.com/api -d '{"key":"value"}'`)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Method != "POST" {
		t.Errorf("method = %q, want POST (implied by -d)", spec.Method)
	}
}

func TestParseNoURL(t *testing.T) {
	_, err := Parse(`curl -H "Accept: text/html"`)
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestShellSplitQuotes(t *testing.T) {
	tokens, err := shellSplit(`one "two three" 'four five' six`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"one", "two three", "four five", "six"}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(want), tokens)
	}
	for i, w := range want {
		if tokens[i] != w {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i], w)
		}
	}
}

func TestDefaultAssertion(t *testing.T) {
	spec, err := Parse(`curl https://example.com`)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Assertions) != 1 {
		t.Fatalf("assertions count = %d, want 1", len(spec.Assertions))
	}
	if spec.Assertions[0].Type != "status" || spec.Assertions[0].Equals != 200 {
		t.Errorf("assertion = %+v, want status==200", spec.Assertions[0])
	}
}
