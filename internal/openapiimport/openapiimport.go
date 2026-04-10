// Package openapiimport parses OpenAPI 3 JSON specs and converts them into
// API Workbench request specifications.
//
// Supports a pragmatic subset of OpenAPI 3:
//   - paths.{path}.{method} → request.Spec
//   - operationId / summary → request name
//   - parameters (query, header, path) → query/headers/URL interpolation
//   - requestBody.content.application/json → JSON body (from example or schema)
//   - responses.200 → status assertion
//
// YAML is not supported (zero-dependency policy). Users can convert YAML → JSON
// before importing.
package openapiimport

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/MakiDevelop/api-workbench/internal/request"
)

// Parse converts an OpenAPI 3 JSON document into a slice of request specs.
// The base URL is taken from servers[0].url if present.
func Parse(data []byte) ([]request.Spec, error) {
	var doc openAPIDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI JSON: %w", err)
	}

	if doc.OpenAPI == "" && doc.Swagger == "" {
		return nil, fmt.Errorf("not an OpenAPI/Swagger document (missing openapi/swagger field)")
	}

	baseURL := ""
	if len(doc.Servers) > 0 {
		baseURL = strings.TrimRight(doc.Servers[0].URL, "/")
	}

	// Sort paths for deterministic output.
	paths := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var specs []request.Spec
	for _, path := range paths {
		pathItem := doc.Paths[path]
		operations := pathItem.Operations()

		// Deterministic method order.
		methods := make([]string, 0, len(operations))
		for m := range operations {
			methods = append(methods, m)
		}
		sort.Strings(methods)

		for _, method := range methods {
			op := operations[method]
			spec := buildSpec(baseURL, path, method, op)
			specs = append(specs, spec)
		}
	}

	if len(specs) == 0 {
		return nil, fmt.Errorf("no operations found in OpenAPI document")
	}

	return specs, nil
}

func buildSpec(baseURL, path, method string, op *operation) request.Spec {
	spec := request.Spec{
		Name:   deriveName(op, method, path),
		Method: strings.ToUpper(method),
		URL:    baseURL + path,
	}

	headers := make(map[string]string)
	query := make(map[string]string)

	for _, p := range op.Parameters {
		switch p.In {
		case "header":
			headers[p.Name] = placeholderFor(p)
		case "query":
			query[p.Name] = placeholderFor(p)
		case "path":
			placeholder := "${" + toEnvVarName(p.Name) + "}"
			spec.URL = strings.ReplaceAll(spec.URL, "{"+p.Name+"}", placeholder)
		}
	}

	if len(headers) > 0 {
		spec.Headers = headers
	}
	if len(query) > 0 {
		spec.Query = query
	}

	// Request body.
	if op.RequestBody != nil {
		if jsonContent, ok := op.RequestBody.Content["application/json"]; ok {
			body := deriveBody(jsonContent)
			if body != nil {
				spec.Body = body
				if spec.Headers == nil {
					spec.Headers = make(map[string]string)
				}
				if _, has := spec.Headers["Content-Type"]; !has {
					spec.Headers["Content-Type"] = "application/json"
				}
			}
		}
	}

	// Status assertion from first 2xx response.
	spec.Assertions = deriveAssertions(op.Responses)

	return spec
}

func deriveName(op *operation, method, path string) string {
	if op.OperationID != "" {
		return op.OperationID
	}
	if op.Summary != "" {
		return sanitize(op.Summary)
	}
	// Fallback: method + path.
	return strings.ToLower(method) + "-" + sanitize(strings.Trim(path, "/"))
}

func deriveBody(content mediaType) *request.Body {
	// Prefer explicit example.
	if content.Example != nil {
		raw, err := json.Marshal(content.Example)
		if err == nil {
			return &request.Body{Type: "json", Content: raw}
		}
	}
	// Examples map (use first).
	if len(content.Examples) > 0 {
		for _, ex := range content.Examples {
			if ex.Value != nil {
				raw, err := json.Marshal(ex.Value)
				if err == nil {
					return &request.Body{Type: "json", Content: raw}
				}
			}
		}
	}
	// Generate from schema (best-effort).
	if content.Schema != nil {
		generated := exampleFromSchema(content.Schema)
		if generated != nil {
			raw, err := json.Marshal(generated)
			if err == nil {
				return &request.Body{Type: "json", Content: raw}
			}
		}
	}
	return nil
}

func exampleFromSchema(schema *schemaObject) any {
	if schema == nil {
		return nil
	}
	if schema.Example != nil {
		return schema.Example
	}
	switch schema.Type {
	case "string":
		return "string"
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		if schema.Items != nil {
			inner := exampleFromSchema(schema.Items)
			return []any{inner}
		}
		return []any{}
	case "object":
		obj := make(map[string]any)
		for k, prop := range schema.Properties {
			obj[k] = exampleFromSchema(prop)
		}
		return obj
	}
	return nil
}

func deriveAssertions(responses map[string]*responseObject) []request.Assertion {
	// Find first 2xx response code.
	codes := make([]string, 0, len(responses))
	for code := range responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	for _, code := range codes {
		if len(code) == 3 && code[0] == '2' {
			if n := parseIntSafe(code); n > 0 {
				return []request.Assertion{{Type: "status", Equals: n}}
			}
		}
	}
	// Default.
	return []request.Assertion{{Type: "status", Equals: 200}}
}

func placeholderFor(p parameter) string {
	if p.Example != nil {
		if s, ok := p.Example.(string); ok {
			return s
		}
		raw, _ := json.Marshal(p.Example)
		return string(raw)
	}
	if p.Schema != nil && p.Schema.Example != nil {
		if s, ok := p.Schema.Example.(string); ok {
			return s
		}
		raw, _ := json.Marshal(p.Schema.Example)
		return string(raw)
	}
	return "${" + toEnvVarName(p.Name) + "}"
}

func toEnvVarName(name string) string {
	var b strings.Builder
	for _, ch := range strings.ToUpper(name) {
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func sanitize(s string) string {
	var b strings.Builder
	for _, ch := range strings.ToLower(s) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			b.WriteRune(ch)
		} else if ch == ' ' || ch == '/' {
			b.WriteRune('-')
		}
	}
	result := b.String()
	result = strings.Trim(result, "-")
	if len(result) > 80 {
		result = result[:80]
	}
	return result
}

func parseIntSafe(s string) int {
	var n int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

// --- OpenAPI document types (minimal subset) ---

type openAPIDoc struct {
	OpenAPI string               `json:"openapi"`
	Swagger string               `json:"swagger"`
	Servers []server             `json:"servers"`
	Paths   map[string]*pathItem `json:"paths"`
}

type server struct {
	URL string `json:"url"`
}

type pathItem struct {
	Get     *operation `json:"get"`
	Post    *operation `json:"post"`
	Put     *operation `json:"put"`
	Patch   *operation `json:"patch"`
	Delete  *operation `json:"delete"`
	Head    *operation `json:"head"`
	Options *operation `json:"options"`
}

func (p *pathItem) Operations() map[string]*operation {
	out := make(map[string]*operation)
	if p.Get != nil {
		out["get"] = p.Get
	}
	if p.Post != nil {
		out["post"] = p.Post
	}
	if p.Put != nil {
		out["put"] = p.Put
	}
	if p.Patch != nil {
		out["patch"] = p.Patch
	}
	if p.Delete != nil {
		out["delete"] = p.Delete
	}
	if p.Head != nil {
		out["head"] = p.Head
	}
	if p.Options != nil {
		out["options"] = p.Options
	}
	return out
}

type operation struct {
	OperationID string                     `json:"operationId"`
	Summary     string                     `json:"summary"`
	Parameters  []parameter                `json:"parameters"`
	RequestBody *requestBodyObject         `json:"requestBody"`
	Responses   map[string]*responseObject `json:"responses"`
}

type parameter struct {
	Name    string        `json:"name"`
	In      string        `json:"in"`
	Example any           `json:"example"`
	Schema  *schemaObject `json:"schema"`
}

type requestBodyObject struct {
	Content map[string]mediaType `json:"content"`
}

type mediaType struct {
	Schema   *schemaObject         `json:"schema"`
	Example  any                   `json:"example"`
	Examples map[string]*exampleVo `json:"examples"`
}

type exampleVo struct {
	Value any `json:"value"`
}

type schemaObject struct {
	Type       string                   `json:"type"`
	Example    any                      `json:"example"`
	Items      *schemaObject            `json:"items"`
	Properties map[string]*schemaObject `json:"properties"`
}

type responseObject struct {
	Description string `json:"description"`
}
