package discover

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RequestFiles finds all .json request spec files under a collection path.
// If the path is a file, it returns that single file.
func RequestFiles(collectionPath string) ([]string, error) {
	info, err := os.Stat(collectionPath)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if filepath.Ext(collectionPath) != ".json" {
			return nil, fmt.Errorf("%s is not a directory or JSON request file", collectionPath)
		}
		return []string{collectionPath}, nil
	}

	var files []string
	err = filepath.WalkDir(collectionPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// EnvNames lists environment names from .apiw/env/*.env files.
func EnvNames(root string) ([]string, error) {
	envDir := filepath.Join(root, ".apiw", "env")
	entries, err := os.ReadDir(envDir)
	if err != nil {
		return nil, err
	}

	var envs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".env" {
			continue
		}
		envs = append(envs, strings.TrimSuffix(name, ".env"))
	}

	sort.Strings(envs)
	return envs, nil
}

// DisplayRelative returns the relative path from root, or the original if Rel fails.
func DisplayRelative(root, value string) string {
	relative, err := filepath.Rel(root, value)
	if err != nil {
		return value
	}
	return relative
}
