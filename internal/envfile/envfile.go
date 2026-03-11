package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Load reads a .env file and returns a variable map.
// File-defined values take precedence over process environment variables.
// Process env is only used as fallback for keys NOT defined in the file.
func Load(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileValues := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d invalid env entry", path, lineNo)
		}

		fileValues[strings.TrimSpace(key)] = unquote(strings.TrimSpace(value))
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Process env is fallback only: file values always win.
	values := make(map[string]string, len(fileValues))
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	for key, value := range fileValues {
		values[key] = value // file wins
	}

	return values, nil
}

// unquote strips matching surrounding quotes (single or double).
func unquote(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
