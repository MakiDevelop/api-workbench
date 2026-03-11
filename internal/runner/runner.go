package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/request"
)

type Options struct {
	Variables map[string]string
	Timeout   time.Duration
	Client    *http.Client // shared client for cookie persistence across collection runs
}

type Result struct {
	Method            string            `json:"method"`
	URL               string            `json:"url"`
	StatusCode        int               `json:"statusCode"`
	Headers           map[string]string `json:"headers"`
	Body              string            `json:"body"`
	Duration          time.Duration     `json:"-"`
	DurationMS        int64             `json:"durationMs"`
	AssertionMessages []string          `json:"assertions"`
	Extracted         map[string]string `json:"extracted,omitempty"`
}

// NewSharedClient creates an http.Client with a cookie jar for use across a collection run.
func NewSharedClient(timeout time.Duration) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: timeout,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme: %s", req.URL.Scheme)
			}
			return nil
		},
	}
}

type Snapshot struct {
	RequestName string            `json:"requestName"`
	Environment string            `json:"environment"`
	CapturedAt  string            `json:"capturedAt"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	StatusCode  int               `json:"statusCode"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	DurationMS  int64             `json:"durationMs"`
}

type AssertionError struct {
	Messages []string
}

func (e *AssertionError) Error() string {
	return strings.Join(e.Messages, "; ")
}

func Run(spec request.Spec, opts Options) (Result, error) {
	expanded, err := expandSpec(spec, opts.Variables)
	if err != nil {
		return Result{}, err
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	client := opts.Client
	if client == nil {
		client = NewSharedClient(timeout)
	}

	reqURL, err := url.Parse(expanded.URL)
	if err != nil {
		return Result{}, err
	}
	if reqURL.Scheme != "http" && reqURL.Scheme != "https" {
		return Result{}, fmt.Errorf("unsupported URL scheme: %s (only http and https are allowed)", reqURL.Scheme)
	}

	query := reqURL.Query()
	for key, value := range expanded.Query {
		query.Set(key, value)
	}
	reqURL.RawQuery = query.Encode()

	bodyBytes, contentType, err := renderBody(expanded.Body)
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequest(strings.ToUpper(expanded.Method), reqURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return Result{}, err
	}

	for key, value := range expanded.Headers {
		req.Header.Set(key, value)
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	started := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	responseBody, err := readBody(resp.Body)
	if err != nil {
		return Result{}, err
	}

	duration := time.Since(started)
	result := Result{
		Method:     req.Method,
		URL:        reqURL.String(),
		StatusCode: resp.StatusCode,
		Headers:    flattenHeaders(resp.Header),
		Body:       string(responseBody),
		Duration:   duration,
		DurationMS: duration.Milliseconds(),
	}

	messages := evaluateAssertions(expanded.Assertions, result)
	result.AssertionMessages = messages

	// Extract values from response for chaining
	if len(spec.Extract) > 0 {
		extracted := make(map[string]string, len(spec.Extract))
		for name, pointer := range spec.Extract {
			val, extractErr := resolveJSONPointer(result.Body, pointer)
			if extractErr != nil {
				return result, fmt.Errorf("extract %q (pointer %s): %w", name, pointer, extractErr)
			}
			extracted[name] = val
		}
		result.Extracted = extracted
	}

	if len(messages) > 0 {
		return result, &AssertionError{Messages: messages}
	}

	return result, nil
}

func WriteSnapshot(root, envName string, spec request.Spec, result Result) (string, error) {
	dir := filepath.Join(root, ".apiw", "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	path := filepath.Join(dir, sanitize(spec.Name)+"--"+sanitize(envName)+".json")
	payload := Snapshot{
		RequestName: spec.Name,
		Environment: envName,
		CapturedAt:  time.Now().UTC().Format(time.RFC3339),
		Method:      result.Method,
		URL:         result.URL,
		StatusCode:  result.StatusCode,
		Headers:     result.Headers,
		Body:        result.Body,
		DurationMS:  result.DurationMS,
	}

	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", err
	}

	return path, nil
}

func expandSpec(spec request.Spec, vars map[string]string) (request.Spec, error) {
	expanded := spec
	var err error

	expanded.URL, err = expandString(spec.URL, vars)
	if err != nil {
		return request.Spec{}, fmt.Errorf("url: %w", err)
	}
	expanded.Headers, err = expandMap(spec.Headers, vars)
	if err != nil {
		return request.Spec{}, fmt.Errorf("headers: %w", err)
	}
	expanded.Query, err = expandMap(spec.Query, vars)
	if err != nil {
		return request.Spec{}, fmt.Errorf("query: %w", err)
	}

	if spec.Body != nil {
		body := *spec.Body
		body.Type, err = expandString(body.Type, vars)
		if err != nil {
			return request.Spec{}, fmt.Errorf("body type: %w", err)
		}
		bodyStr, err := expandString(string(body.Content), vars)
		if err != nil {
			return request.Spec{}, fmt.Errorf("body content: %w", err)
		}
		body.Content = json.RawMessage(bodyStr)
		expanded.Body = &body
	}

	return expanded, nil
}

func expandMap(input map[string]string, vars map[string]string) (map[string]string, error) {
	if len(input) == 0 {
		return nil, nil
	}

	output := make(map[string]string, len(input))
	for key, value := range input {
		expanded, err := expandString(value, vars)
		if err != nil {
			return nil, fmt.Errorf("key %s: %w", key, err)
		}
		output[key] = expanded
	}
	return output, nil
}

func expandString(value string, vars map[string]string) (string, error) {
	var missing []string
	result := os.Expand(value, func(key string) string {
		if v, ok := vars[key]; ok {
			return v
		}
		missing = append(missing, key)
		return ""
	})
	if len(missing) > 0 {
		return result, fmt.Errorf("undefined variable(s): %s", strings.Join(missing, ", "))
	}
	return result, nil
}

func renderBody(body *request.Body) ([]byte, string, error) {
	if body == nil {
		return nil, "", nil
	}

	bodyType := strings.ToLower(strings.TrimSpace(body.Type))
	switch bodyType {
	case "", "json":
		return normalizeJSON(body.Content)
	case "text":
		var text string
		if err := json.Unmarshal(body.Content, &text); err != nil {
			return nil, "", fmt.Errorf("text body must be a JSON string")
		}
		return []byte(text), "text/plain; charset=utf-8", nil
	case "form":
		return renderFormBody(body.Content)
	default:
		return nil, "", fmt.Errorf("unsupported body type: %s", body.Type)
	}
}

func renderFormBody(content json.RawMessage) ([]byte, string, error) {
	var fields map[string]string
	if err := json.Unmarshal(content, &fields); err != nil {
		return nil, "", fmt.Errorf("form body must be a JSON object with string values: %w", err)
	}

	form := url.Values{}
	for key, value := range fields {
		form.Set(key, value)
	}

	return []byte(form.Encode()), "application/x-www-form-urlencoded", nil
}

func normalizeJSON(raw json.RawMessage) ([]byte, string, error) {
	if len(raw) == 0 {
		return nil, "application/json", nil
	}

	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, "", fmt.Errorf("invalid JSON body: %w", err)
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}

	return encoded, "application/json", nil
}

func flattenHeaders(header http.Header) map[string]string {
	flattened := make(map[string]string, len(header))
	for key, values := range header {
		flattened[key] = strings.Join(values, ", ")
	}
	return flattened
}

func sanitize(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "@", "-", ".", "-")
	return replacer.Replace(value)
}

func evaluateAssertions(assertions []request.Assertion, result Result) []string {
	var failures []string

	for _, assertion := range assertions {
		switch assertion.Type {
		case "status":
			if result.StatusCode != assertion.Equals {
				failures = append(failures, fmt.Sprintf("expected status %d, got %d", assertion.Equals, result.StatusCode))
			}
		case "body_contains":
			if !strings.Contains(result.Body, assertion.Contains) {
				failures = append(failures, fmt.Sprintf("expected body to contain %q", assertion.Contains))
			}
		case "header_equals":
			got := lookupHeader(result.Headers, assertion.Key)
			if got != assertion.Value {
				failures = append(failures, fmt.Sprintf("expected header %s=%q, got %q", assertion.Key, assertion.Value, got))
			}
		case "json_path":
			got, err := resolveJSONPointer(result.Body, assertion.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("json_path %s: %v", assertion.Path, err))
			} else if got != assertion.Expected {
				failures = append(failures, fmt.Sprintf("json_path %s: expected %q, got %q", assertion.Path, assertion.Expected, got))
			}
		case "body_regex":
			matched, err := regexp.MatchString(assertion.Pattern, result.Body)
			if err != nil {
				failures = append(failures, fmt.Sprintf("body_regex: invalid pattern %q: %v", assertion.Pattern, err))
			} else if !matched {
				failures = append(failures, fmt.Sprintf("body did not match pattern %q", assertion.Pattern))
			}
		case "duration_under":
			if result.DurationMS > int64(assertion.Under) {
				failures = append(failures, fmt.Sprintf("expected duration under %dms, got %dms", assertion.Under, result.DurationMS))
			}
		case "json_path_count":
			count, err := resolveJSONArrayLength(result.Body, assertion.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("json_path_count %s: %v", assertion.Path, err))
			} else if count != assertion.Equals {
				failures = append(failures, fmt.Sprintf("json_path_count %s: expected %d items, got %d", assertion.Path, assertion.Equals, count))
			}
		}
	}

	return failures
}

// lookupHeader does a case-insensitive header lookup.
func lookupHeader(headers map[string]string, key string) string {
	if v, ok := headers[key]; ok {
		return v
	}
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

// resolveJSONPointer evaluates an RFC 6901 JSON Pointer against a JSON body.
// e.g. "/user/name" on {"user":{"name":"alice"}} returns "alice".
func resolveJSONPointer(body string, pointer string) (string, error) {
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return "", fmt.Errorf("body is not valid JSON: %w", err)
	}

	if pointer == "" || pointer == "/" {
		return fmt.Sprintf("%v", data), nil
	}

	if !strings.HasPrefix(pointer, "/") {
		return "", fmt.Errorf("JSON pointer must start with /")
	}

	parts := strings.Split(pointer[1:], "/")
	current := data

	for _, part := range parts {
		// RFC 6901 unescaping
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		switch node := current.(type) {
		case map[string]any:
			val, ok := node[part]
			if !ok {
				return "", fmt.Errorf("key %q not found", part)
			}
			current = val
		case []any:
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err != nil {
				return "", fmt.Errorf("expected array index, got %q", part)
			}
			if idx < 0 || idx >= len(node) {
				return "", fmt.Errorf("array index %d out of range (length %d)", idx, len(node))
			}
			current = node[idx]
		default:
			return "", fmt.Errorf("cannot traverse into %T at %q", current, part)
		}
	}

	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), nil
		}
		return fmt.Sprintf("%g", v), nil
	case bool:
		return fmt.Sprintf("%t", v), nil
	case nil:
		return "null", nil
	default:
		raw, _ := json.Marshal(v)
		return string(raw), nil
	}
}

// resolveJSONArrayLength evaluates a JSON Pointer and returns the length of the array at that path.
func resolveJSONArrayLength(body string, pointer string) (int, error) {
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return 0, fmt.Errorf("body is not valid JSON: %w", err)
	}

	current := data

	if pointer != "" && pointer != "/" {
		if !strings.HasPrefix(pointer, "/") {
			return 0, fmt.Errorf("JSON pointer must start with /")
		}

		parts := strings.Split(pointer[1:], "/")
		for _, part := range parts {
			part = strings.ReplaceAll(part, "~1", "/")
			part = strings.ReplaceAll(part, "~0", "~")

			switch node := current.(type) {
			case map[string]any:
				val, ok := node[part]
				if !ok {
					return 0, fmt.Errorf("key %q not found", part)
				}
				current = val
			case []any:
				var idx int
				if _, err := fmt.Sscanf(part, "%d", &idx); err != nil {
					return 0, fmt.Errorf("expected array index, got %q", part)
				}
				if idx < 0 || idx >= len(node) {
					return 0, fmt.Errorf("array index %d out of range (length %d)", idx, len(node))
				}
				current = node[idx]
			default:
				return 0, fmt.Errorf("cannot traverse into %T at %q", current, part)
			}
		}
	}

	arr, ok := current.([]any)
	if !ok {
		return 0, fmt.Errorf("value at %q is not an array", pointer)
	}
	return len(arr), nil
}
