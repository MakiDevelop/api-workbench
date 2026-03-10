package project

import (
	"fmt"
	"os"
	"path/filepath"
)

func FindRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		config := filepath.Join(current, ".apiw", "apiw.json")
		if _, err := os.Stat(config); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("could not find .apiw/apiw.json from %s", start)
		}
		current = parent
	}
}
