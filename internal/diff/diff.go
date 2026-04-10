package diff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SnapshotDiff represents the structural difference between two snapshots.
type SnapshotDiff struct {
	LeftTime  string       `json:"leftTime"`
	RightTime string       `json:"rightTime"`
	Same      bool         `json:"same"`
	Changes   []ChangeItem `json:"changes"`
}

// ChangeItem represents a single field-level change.
type ChangeItem struct {
	Field string `json:"field"` // e.g. "statusCode", "headers.Content-Type", "body"
	Type  string `json:"type"`  // "changed", "added", "removed"
	Left  string `json:"left"`  // old value (empty if added)
	Right string `json:"right"` // new value (empty if removed)
}

// Snapshots compares two JSON snapshot files and returns structured differences.
func Snapshots(leftJSON, rightJSON []byte) (SnapshotDiff, error) {
	var left, right map[string]any
	if err := json.Unmarshal(leftJSON, &left); err != nil {
		return SnapshotDiff{}, fmt.Errorf("left snapshot: %w", err)
	}
	if err := json.Unmarshal(rightJSON, &right); err != nil {
		return SnapshotDiff{}, fmt.Errorf("right snapshot: %w", err)
	}

	result := SnapshotDiff{
		LeftTime:  stringVal(left["capturedAt"]),
		RightTime: stringVal(right["capturedAt"]),
	}

	// Compare scalar fields.
	for _, field := range []string{"requestName", "environment", "method", "url", "statusCode", "durationMs"} {
		l := formatAny(left[field])
		r := formatAny(right[field])
		if l != r {
			result.Changes = append(result.Changes, ChangeItem{
				Field: field,
				Type:  "changed",
				Left:  l,
				Right: r,
			})
		}
	}

	// Compare headers.
	leftHeaders := asStringMap(left["headers"])
	rightHeaders := asStringMap(right["headers"])
	result.Changes = append(result.Changes, diffMaps("headers", leftHeaders, rightHeaders)...)

	// Compare body.
	leftBody := stringVal(left["body"])
	rightBody := stringVal(right["body"])
	if leftBody != rightBody {
		// Try to do structured JSON diff if both are JSON.
		leftPretty := prettyJSON(leftBody)
		rightPretty := prettyJSON(rightBody)

		result.Changes = append(result.Changes, ChangeItem{
			Field: "body",
			Type:  "changed",
			Left:  truncate(leftPretty, 5000),
			Right: truncate(rightPretty, 5000),
		})
	}

	result.Same = len(result.Changes) == 0

	return result, nil
}

func diffMaps(prefix string, left, right map[string]string) []ChangeItem {
	var changes []ChangeItem

	// Collect all keys.
	allKeys := make(map[string]bool)
	for k := range left {
		allKeys[k] = true
	}
	for k := range right {
		allKeys[k] = true
	}

	sorted := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, k := range sorted {
		lv, lOK := left[k]
		rv, rOK := right[k]
		field := prefix + "." + k

		switch {
		case lOK && rOK && lv != rv:
			changes = append(changes, ChangeItem{Field: field, Type: "changed", Left: lv, Right: rv})
		case lOK && !rOK:
			changes = append(changes, ChangeItem{Field: field, Type: "removed", Left: lv})
		case !lOK && rOK:
			changes = append(changes, ChangeItem{Field: field, Type: "added", Right: rv})
		}
	}

	return changes
}

func stringVal(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return formatAny(v)
}

func formatAny(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		raw, _ := json.Marshal(val)
		return string(raw)
	}
}

func asStringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, val := range m {
		result[k] = stringVal(val)
	}
	return result
}

func prettyJSON(s string) string {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		formatted, err := json.MarshalIndent(v, "", "  ")
		if err == nil {
			return string(formatted)
		}
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}

// Lines produces a unified-diff-style text representation of the changes.
func Lines(d SnapshotDiff) string {
	if d.Same {
		return "No differences found."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n", d.LeftTime)
	fmt.Fprintf(&b, "+++ %s\n", d.RightTime)
	fmt.Fprintf(&b, "%d change(s)\n\n", len(d.Changes))

	for _, c := range d.Changes {
		switch c.Type {
		case "changed":
			fmt.Fprintf(&b, "~ %s\n", c.Field)
			fmt.Fprintf(&b, "  - %s\n", c.Left)
			fmt.Fprintf(&b, "  + %s\n\n", c.Right)
		case "added":
			fmt.Fprintf(&b, "+ %s: %s\n\n", c.Field, c.Right)
		case "removed":
			fmt.Fprintf(&b, "- %s: %s\n\n", c.Field, c.Left)
		}
	}

	return b.String()
}
