package request

import (
	"encoding/json"
	"fmt"
	"os"
)

type Spec struct {
	Name       string            `json:"name"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	Query      map[string]string `json:"query"`
	Assertions []Assertion       `json:"assertions"`
	Body       *Body             `json:"body"`
}

type Body struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

type Assertion struct {
	Type     string `json:"type"`
	Equals   int    `json:"equals,omitempty"`
	Contains string `json:"contains,omitempty"`
	Key      string `json:"key,omitempty"`
	Value    string `json:"value,omitempty"`
}

func Load(path string) (Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}

	var spec Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return Spec{}, err
	}

	if err := spec.Validate(); err != nil {
		return Spec{}, fmt.Errorf("%s: %w", path, err)
	}

	return spec, nil
}

func (s Spec) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	if s.Method == "" {
		return fmt.Errorf("method is required")
	}
	if s.URL == "" {
		return fmt.Errorf("url is required")
	}

	for _, assertion := range s.Assertions {
		switch assertion.Type {
		case "status":
			if assertion.Equals == 0 {
				return fmt.Errorf("status assertion requires equals")
			}
		case "body_contains":
			if assertion.Contains == "" {
				return fmt.Errorf("body_contains assertion requires contains")
			}
		case "header_equals":
			if assertion.Key == "" || assertion.Value == "" {
				return fmt.Errorf("header_equals assertion requires key and value")
			}
		default:
			return fmt.Errorf("unsupported assertion type: %s", assertion.Type)
		}
	}

	return nil
}
