package curlimport

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/MakiDevelop/api-workbench/internal/request"
)

// Parse converts a cURL command string into a request.Spec.
// It supports the most common cURL flags:
//   -X/--request METHOD
//   -H/--header "Key: Value"
//   -d/--data/--data-raw BODY
//   -F/--form "field=value"
//   -u/--user "user:pass"
//   URL (positional argument)
func Parse(curlCmd string) (request.Spec, error) {
	tokens, err := shellSplit(curlCmd)
	if err != nil {
		return request.Spec{}, fmt.Errorf("parse error: %w", err)
	}

	if len(tokens) == 0 {
		return request.Spec{}, fmt.Errorf("empty command")
	}

	// Strip leading "curl" if present.
	if strings.ToLower(tokens[0]) == "curl" {
		tokens = tokens[1:]
	}

	var (
		method      string
		rawURL      string
		headers     = make(map[string]string)
		dataValues  []string
		formValues  []string
		basicAuth   string
	)

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]

		switch {
		case tok == "-X" || tok == "--request":
			i++
			if i >= len(tokens) {
				return request.Spec{}, fmt.Errorf("-X requires a method argument")
			}
			method = strings.ToUpper(tokens[i])

		case tok == "-H" || tok == "--header":
			i++
			if i >= len(tokens) {
				return request.Spec{}, fmt.Errorf("-H requires a header argument")
			}
			key, val, ok := parseHeader(tokens[i])
			if ok {
				headers[key] = val
			}

		case tok == "-d" || tok == "--data" || tok == "--data-raw" || tok == "--data-binary":
			i++
			if i >= len(tokens) {
				return request.Spec{}, fmt.Errorf("%s requires a data argument", tok)
			}
			dataValues = append(dataValues, tokens[i])

		case tok == "-F" || tok == "--form":
			i++
			if i >= len(tokens) {
				return request.Spec{}, fmt.Errorf("-F requires a form argument")
			}
			formValues = append(formValues, tokens[i])

		case tok == "-u" || tok == "--user":
			i++
			if i >= len(tokens) {
				return request.Spec{}, fmt.Errorf("-u requires a user:pass argument")
			}
			basicAuth = tokens[i]

		case tok == "--url":
			i++
			if i >= len(tokens) {
				return request.Spec{}, fmt.Errorf("--url requires a URL argument")
			}
			rawURL = tokens[i]

		case strings.HasPrefix(tok, "-"):
			// Skip unknown flags — some take an argument, some don't.
			// Heuristic: if the next token doesn't start with "-" and we have
			// no URL yet, it might be the flag's argument.
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") && !looksLikeURL(tokens[i+1]) {
				i++ // skip argument of unknown flag
			}

		default:
			// Positional argument — treat as URL.
			if rawURL == "" {
				rawURL = tok
			}
		}
	}

	if rawURL == "" {
		return request.Spec{}, fmt.Errorf("no URL found in cURL command")
	}

	// Parse URL and extract query params.
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return request.Spec{}, fmt.Errorf("invalid URL: %w", err)
	}

	query := make(map[string]string)
	for key, values := range parsedURL.Query() {
		if len(values) > 0 {
			query[key] = values[len(values)-1]
		}
	}
	// Reconstruct URL without query string.
	parsedURL.RawQuery = ""
	cleanURL := parsedURL.String()

	// Determine method.
	if method == "" {
		if len(dataValues) > 0 || len(formValues) > 0 {
			method = "POST"
		} else {
			method = "GET"
		}
	}

	// Handle basic auth.
	if basicAuth != "" {
		encoded := basicAuthHeader(basicAuth)
		headers["Authorization"] = "Basic " + encoded
	}

	// Build spec.
	spec := request.Spec{
		Name:   deriveName(parsedURL),
		Method: method,
		URL:    cleanURL,
	}

	if len(headers) > 0 {
		spec.Headers = headers
	}
	if len(query) > 0 {
		spec.Query = query
	}

	// Build body.
	if len(formValues) > 0 {
		formMap := make(map[string]string)
		for _, fv := range formValues {
			k, v, _ := strings.Cut(fv, "=")
			formMap[k] = v
		}
		content, _ := json.Marshal(formMap)
		spec.Body = &request.Body{
			Type:    "form",
			Content: content,
		}
	} else if len(dataValues) > 0 {
		combined := strings.Join(dataValues, "")
		// Check if the data is valid JSON.
		var jsonCheck json.RawMessage
		if json.Unmarshal([]byte(combined), &jsonCheck) == nil {
			// Pretty-format the JSON for readability.
			var buf []byte
			buf, _ = json.Marshal(jsonCheck)
			spec.Body = &request.Body{
				Type:    "json",
				Content: buf,
			}
			// Auto-set Content-Type if not already set.
			if _, ok := headers["Content-Type"]; !ok {
				if spec.Headers == nil {
					spec.Headers = make(map[string]string)
				}
				spec.Headers["Content-Type"] = "application/json"
			}
		} else {
			content, _ := json.Marshal(combined)
			spec.Body = &request.Body{
				Type:    "text",
				Content: content,
			}
		}
	}

	// Add a default status assertion.
	spec.Assertions = []request.Assertion{
		{Type: "status", Equals: 200},
	}

	return spec, nil
}

func parseHeader(raw string) (key, value string, ok bool) {
	k, v, found := strings.Cut(raw, ":")
	if !found {
		return "", "", false
	}
	return strings.TrimSpace(k), strings.TrimSpace(v), true
}

func deriveName(u *url.URL) string {
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return u.Host
	}
	parts := strings.Split(path, "/")
	// Use last 2 path segments for a descriptive name.
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	name := strings.Join(parts, "-")
	// Replace non-alphanumeric chars.
	var b strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			b.WriteRune(ch)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.Contains(s, "://")
}

func basicAuthHeader(userPass string) string {
	// Base64 encode without importing encoding/base64 — use a simple implementation.
	return base64Encode([]byte(userPass))
}

const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func base64Encode(data []byte) string {
	var b strings.Builder
	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i
		switch {
		case remaining >= 3:
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			b.WriteByte(base64Chars[n>>18&0x3F])
			b.WriteByte(base64Chars[n>>12&0x3F])
			b.WriteByte(base64Chars[n>>6&0x3F])
			b.WriteByte(base64Chars[n&0x3F])
		case remaining == 2:
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8
			b.WriteByte(base64Chars[n>>18&0x3F])
			b.WriteByte(base64Chars[n>>12&0x3F])
			b.WriteByte(base64Chars[n>>6&0x3F])
			b.WriteByte('=')
		case remaining == 1:
			n = uint32(data[i]) << 16
			b.WriteByte(base64Chars[n>>18&0x3F])
			b.WriteByte(base64Chars[n>>12&0x3F])
			b.WriteByte('=')
			b.WriteByte('=')
		}
	}
	return b.String()
}

// shellSplit splits a command string into tokens, respecting single and double
// quotes, backslash escapes, and line continuations (trailing backslash).
func shellSplit(s string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	hasContent := false

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		if ch == '\\' && !inSingle {
			if i+1 < len(runes) {
				next := runes[i+1]
				if next == '\n' {
					// Line continuation — skip both characters.
					i++
					continue
				}
				if inDouble {
					// In double quotes, only certain chars are escaped.
					if next == '"' || next == '\\' || next == '$' || next == '`' {
						current.WriteRune(next)
						i++
						hasContent = true
						continue
					}
				} else {
					// Outside quotes, backslash escapes the next character.
					current.WriteRune(next)
					i++
					hasContent = true
					continue
				}
			}
		}

		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			hasContent = true
			continue
		}

		if ch == '"' && !inSingle {
			inDouble = !inDouble
			hasContent = true
			continue
		}

		if !inSingle && !inDouble && (ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r') {
			if hasContent || current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
				hasContent = false
			}
			continue
		}

		current.WriteRune(ch)
		hasContent = true
	}

	if inSingle {
		return nil, fmt.Errorf("unterminated single quote")
	}
	if inDouble {
		return nil, fmt.Errorf("unterminated double quote")
	}

	if hasContent || current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}
