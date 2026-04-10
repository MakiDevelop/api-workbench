package runner

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

	// Parse response body JSON once for assertions and extraction.
	var parsedBody any
	var bodyIsJSON bool
	if len(result.Body) > 0 {
		if json.Unmarshal([]byte(result.Body), &parsedBody) == nil {
			bodyIsJSON = true
		}
	}

	messages := evaluateAssertions(expanded.Assertions, result, parsedBody, bodyIsJSON)
	result.AssertionMessages = messages

	// Extract values from response for chaining
	if len(spec.Extract) > 0 {
		if !bodyIsJSON {
			return result, fmt.Errorf("extract requires JSON response body")
		}
		extracted := make(map[string]string, len(spec.Extract))
		for name, pointer := range spec.Extract {
			val, extractErr := resolveJSONPointerParsed(parsedBody, pointer)
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

	// Archive the previous snapshot before overwriting.
	archiveSnapshot(dir, path, spec.Name, envName)

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

	// Resolve auth preset to headers/query.
	if spec.Auth != nil {
		if expanded.Headers == nil {
			expanded.Headers = make(map[string]string)
		}
		auth := spec.Auth
		switch strings.ToLower(auth.Type) {
		case "bearer":
			token, expandErr := expandString(auth.Token, vars)
			if expandErr != nil {
				return request.Spec{}, fmt.Errorf("auth token: %w", expandErr)
			}
			expanded.Headers["Authorization"] = "Bearer " + token
		case "basic":
			user, expandErr := expandString(auth.User, vars)
			if expandErr != nil {
				return request.Spec{}, fmt.Errorf("auth user: %w", expandErr)
			}
			pass, expandErr := expandString(auth.Pass, vars)
			if expandErr != nil {
				return request.Spec{}, fmt.Errorf("auth pass: %w", expandErr)
			}
			encoded := basicAuthEncode(user + ":" + pass)
			expanded.Headers["Authorization"] = "Basic " + encoded
		case "api-key":
			key := auth.Key
			if key == "" {
				key = "X-API-Key"
			}
			val, expandErr := expandString(auth.Value, vars)
			if expandErr != nil {
				return request.Spec{}, fmt.Errorf("auth value: %w", expandErr)
			}
			if strings.ToLower(auth.In) == "query" {
				if expanded.Query == nil {
					expanded.Query = make(map[string]string)
				}
				expanded.Query[key] = val
			} else {
				expanded.Headers[key] = val
			}
		}
		expanded.Auth = nil // auth is resolved, don't pass it further
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
		// Built-in template functions (prefix __).
		if strings.HasPrefix(key, "__") {
			resolved, ok := evalBuiltin(key, vars)
			if ok {
				return resolved
			}
			missing = append(missing, key)
			return ""
		}
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

// evalBuiltin resolves built-in template functions.
// Supported: __now, __timestamp, __uuid, __randomInt[:max],
// __base64:VAR, __sha256:VAR, __hmac_sha256:KEY_VAR:MSG_VAR
func evalBuiltin(key string, vars map[string]string) (string, bool) {
	// Strip __ prefix.
	name := strings.TrimPrefix(key, "__")

	// Split on ":" for function arguments.
	parts := strings.Split(name, ":")
	fn := parts[0]
	args := parts[1:]

	switch fn {
	case "now":
		return time.Now().UTC().Format(time.RFC3339), true
	case "timestamp":
		return fmt.Sprintf("%d", time.Now().Unix()), true
	case "timestampMs":
		return fmt.Sprintf("%d", time.Now().UnixMilli()), true
	case "uuid":
		return generateUUID(), true
	case "randomInt":
		max := 1000000
		if len(args) >= 1 {
			if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
				max = n
			}
		}
		return fmt.Sprintf("%d", randomInt(max)), true
	case "base64":
		if len(args) < 1 {
			return "", false
		}
		src := lookupVarOrLiteral(args[0], vars)
		return basicAuthEncode(src), true
	case "sha256":
		if len(args) < 1 {
			return "", false
		}
		src := lookupVarOrLiteral(args[0], vars)
		return sha256Hex(src), true
	case "hmac_sha256":
		if len(args) < 2 {
			return "", false
		}
		keyVal := lookupVarOrLiteral(args[0], vars)
		msgVal := lookupVarOrLiteral(args[1], vars)
		return hmacSHA256Hex(keyVal, msgVal), true
	}
	return "", false
}

func lookupVarOrLiteral(name string, vars map[string]string) string {
	if v, ok := vars[name]; ok {
		return v
	}
	return name
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

// archiveSnapshot renames the existing snapshot to a timestamped version.
// Keeps up to 10 historical snapshots per request/env pair.
func archiveSnapshot(dir, currentPath, name, envName string) {
	data, err := os.ReadFile(currentPath)
	if err != nil {
		return // no previous snapshot to archive
	}

	// Extract capturedAt from the existing snapshot for the archive name.
	var existing Snapshot
	if json.Unmarshal(data, &existing) != nil {
		return
	}

	// Parse the timestamp and format as compact string.
	ts, err := time.Parse(time.RFC3339, existing.CapturedAt)
	if err != nil {
		ts = time.Now().UTC()
	}
	stamp := ts.Format("20060102-150405")

	archiveName := sanitize(name) + "--" + sanitize(envName) + "--" + stamp + ".json"
	archivePath := filepath.Join(dir, archiveName)

	// Don't overwrite if archive already exists (same timestamp).
	if _, err := os.Stat(archivePath); err == nil {
		return
	}

	os.Rename(currentPath, archivePath)

	// Prune old archives: keep only the latest 10.
	pruneArchives(dir, name, envName, 10)
}

func pruneArchives(dir, name, envName string, keep int) {
	prefix := sanitize(name) + "--" + sanitize(envName) + "--"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var archives []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".json") {
			archives = append(archives, e.Name())
		}
	}

	if len(archives) <= keep {
		return
	}

	// Archives are sorted chronologically by name (timestamp in name).
	sort.Strings(archives)
	for _, a := range archives[:len(archives)-keep] {
		os.Remove(filepath.Join(dir, a))
	}
}

func evaluateAssertions(assertions []request.Assertion, result Result, parsedBody any, bodyIsJSON bool) []string {
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
			if !bodyIsJSON {
				failures = append(failures, fmt.Sprintf("json_path %s: body is not valid JSON", assertion.Path))
			} else {
				got, err := resolveJSONPointerParsed(parsedBody, assertion.Path)
				if err != nil {
					failures = append(failures, fmt.Sprintf("json_path %s: %v", assertion.Path, err))
				} else if got != assertion.Expected {
					failures = append(failures, fmt.Sprintf("json_path %s: expected %q, got %q", assertion.Path, assertion.Expected, got))
				}
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
			if !bodyIsJSON {
				failures = append(failures, fmt.Sprintf("json_path_count %s: body is not valid JSON", assertion.Path))
			} else {
				count, err := resolveJSONArrayLengthParsed(parsedBody, assertion.Path)
				if err != nil {
					failures = append(failures, fmt.Sprintf("json_path_count %s: %v", assertion.Path, err))
				} else if count != assertion.Equals {
					failures = append(failures, fmt.Sprintf("json_path_count %s: expected %d items, got %d", assertion.Path, assertion.Equals, count))
				}
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

// traversePointer walks a pre-parsed JSON value using an RFC 6901 JSON Pointer.
// data may be nil for a valid JSON null body.
func traversePointer(data any, pointer string) (any, error) {
	if pointer == "" || pointer == "/" {
		return data, nil
	}

	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("JSON pointer must start with /")
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
				return nil, fmt.Errorf("key %q not found", part)
			}
			current = val
		case []any:
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err != nil {
				return nil, fmt.Errorf("expected array index, got %q", part)
			}
			if idx < 0 || idx >= len(node) {
				return nil, fmt.Errorf("array index %d out of range (length %d)", idx, len(node))
			}
			current = node[idx]
		default:
			return nil, fmt.Errorf("cannot traverse into %T at %q", current, part)
		}
	}

	return current, nil
}

func formatValue(current any) string {
	switch v := current.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "null"
	default:
		raw, _ := json.Marshal(v)
		return string(raw)
	}
}

// resolveJSONPointerParsed evaluates an RFC 6901 JSON Pointer against pre-parsed data.
func resolveJSONPointerParsed(data any, pointer string) (string, error) {
	current, err := traversePointer(data, pointer)
	if err != nil {
		return "", err
	}
	return formatValue(current), nil
}

// resolveJSONArrayLengthParsed evaluates a JSON Pointer on pre-parsed data and returns the array length.
func resolveJSONArrayLengthParsed(data any, pointer string) (int, error) {
	current, err := traversePointer(data, pointer)
	if err != nil {
		return 0, err
	}
	arr, ok := current.([]any)
	if !ok {
		return 0, fmt.Errorf("value at %q is not an array", pointer)
	}
	return len(arr), nil
}

// resolveJSONPointer evaluates an RFC 6901 JSON Pointer against a JSON body string.
// Kept for backward compatibility with tests.
func resolveJSONPointer(body string, pointer string) (string, error) {
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return "", fmt.Errorf("body is not valid JSON: %w", err)
	}
	return resolveJSONPointerParsed(data, pointer)
}

// resolveJSONArrayLength evaluates a JSON Pointer and returns the length of the array at that path.
// Kept for backward compatibility with tests.
func resolveJSONArrayLength(body string, pointer string) (int, error) {
	var data any
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return 0, fmt.Errorf("body is not valid JSON: %w", err)
	}
	return resolveJSONArrayLengthParsed(data, pointer)
}

func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func randomInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

func sha256Hex(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}

func hmacSHA256Hex(key, msg string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

const b64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func basicAuthEncode(data string) string {
	src := []byte(data)
	var b strings.Builder
	for i := 0; i < len(src); i += 3 {
		remaining := len(src) - i
		var n uint32
		switch {
		case remaining >= 3:
			n = uint32(src[i])<<16 | uint32(src[i+1])<<8 | uint32(src[i+2])
			b.WriteByte(b64Chars[n>>18&0x3F])
			b.WriteByte(b64Chars[n>>12&0x3F])
			b.WriteByte(b64Chars[n>>6&0x3F])
			b.WriteByte(b64Chars[n&0x3F])
		case remaining == 2:
			n = uint32(src[i])<<16 | uint32(src[i+1])<<8
			b.WriteByte(b64Chars[n>>18&0x3F])
			b.WriteByte(b64Chars[n>>12&0x3F])
			b.WriteByte(b64Chars[n>>6&0x3F])
			b.WriteByte('=')
		case remaining == 1:
			n = uint32(src[i]) << 16
			b.WriteByte(b64Chars[n>>18&0x3F])
			b.WriteByte(b64Chars[n>>12&0x3F])
			b.WriteByte('=')
			b.WriteByte('=')
		}
	}
	return b.String()
}
