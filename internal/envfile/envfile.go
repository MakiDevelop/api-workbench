package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Load reads a .env file and returns a variable map.
// Only variables explicitly defined in the file are returned.
// Process environment variables are NOT inherited — this prevents
// untrusted workspaces from exfiltrating secrets like AWS_SECRET_ACCESS_KEY.
func Load(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
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

		values[strings.TrimSpace(key)] = unquote(strings.TrimSpace(value))
	}

	if err := scanner.Err(); err != nil {
		return nil, err
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
