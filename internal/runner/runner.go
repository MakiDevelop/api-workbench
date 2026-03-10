package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/request"
)

type Options struct {
	Variables map[string]string
	Timeout   time.Duration
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

	client := &http.Client{Timeout: timeout}

	reqURL, err := url.Parse(expanded.URL)
	if err != nil {
		return Result{}, err
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
	expanded.URL = expandString(spec.URL, vars)
	expanded.Headers = expandMap(spec.Headers, vars)
	expanded.Query = expandMap(spec.Query, vars)

	if spec.Body != nil {
		body := *spec.Body
		body.Type = expandString(body.Type, vars)
		body.Content = json.RawMessage(expandBytes(body.Content, vars))
		expanded.Body = &body
	}

	return expanded, nil
}

func expandMap(input map[string]string, vars map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}

	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = expandString(value, vars)
	}
	return output
}

func expandString(value string, vars map[string]string) string {
	return os.Expand(value, func(key string) string {
		if value, ok := vars[key]; ok {
			return value
		}
		return ""
	})
}

func expandBytes(value []byte, vars map[string]string) []byte {
	return []byte(expandString(string(value), vars))
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
	default:
		return nil, "", fmt.Errorf("unsupported body type: %s", body.Type)
	}
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
			if result.Headers[assertion.Key] != assertion.Value {
				failures = append(failures, fmt.Sprintf("expected header %s=%q, got %q", assertion.Key, assertion.Value, result.Headers[assertion.Key]))
			}
		}
	}

	return failures
}
