package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

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

		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}

	return values, nil
}
