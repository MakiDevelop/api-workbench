package app

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/project.json
var defaultProjectConfig string

//go:embed templates/local.env
var defaultEnvFile string

//go:embed templates/health.json
var defaultRequest string

func runInit(args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("init does not accept positional arguments")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	dirs := []string{
		filepath.Join(cwd, ".apiw"),
		filepath.Join(cwd, ".apiw", "env"),
		filepath.Join(cwd, ".apiw", "snapshots"),
		filepath.Join(cwd, "requests"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "created dir    %s\n", rel(cwd, dir))
	}

	files := []struct {
		path    string
		content string
	}{
		{path: filepath.Join(cwd, ".apiw", "apiw.json"), content: defaultProjectConfig},
		{path: filepath.Join(cwd, ".apiw", "env", "local.env"), content: defaultEnvFile},
		{path: filepath.Join(cwd, "requests", "health.json"), content: defaultRequest},
	}

	for _, file := range files {
		if _, err := os.Stat(file.path); err == nil {
			fmt.Fprintf(stdout, "skipped file   %s\n", rel(cwd, file.path))
			continue
		} else if !os.IsNotExist(err) {
			return err
		}

		if err := os.WriteFile(file.path, []byte(strings.TrimSpace(file.content)+"\n"), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "created file   %s\n", rel(cwd, file.path))
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "project ready")
	fmt.Fprintln(stdout, "next: apiw run requests/health.json --env local")

	return nil
}

func rel(root, target string) string {
	value, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return value
}
