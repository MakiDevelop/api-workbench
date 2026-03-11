package app

import (
	"embed"
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

//go:embed templates/demo
var demoFS embed.FS

func runInit(args []string, stdout io.Writer) error {
	demo := false
	for _, arg := range args {
		if arg == "--demo" {
			demo = true
		} else {
			return fmt.Errorf("unknown argument: %s", arg)
		}
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

	if demo {
		files = append(files, demoFiles(cwd)...)
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
	if demo {
		fmt.Fprintln(stdout, "next: apiw run --all --env demo")
	} else {
		fmt.Fprintln(stdout, "next: apiw run requests/health.json --env local")
	}

	return nil
}

func demoFiles(cwd string) []struct {
	path    string
	content string
} {
	entries, err := demoFS.ReadDir("templates/demo")
	if err != nil {
		return nil
	}

	var files []struct {
		path    string
		content string
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := demoFS.ReadFile("templates/demo/" + entry.Name())
		if err != nil {
			continue
		}

		var destPath string
		if strings.HasSuffix(entry.Name(), ".env") {
			destPath = filepath.Join(cwd, ".apiw", "env", entry.Name())
		} else {
			destPath = filepath.Join(cwd, "requests", entry.Name())
		}

		files = append(files, struct {
			path    string
			content string
		}{path: destPath, content: string(data)})
	}

	return files
}

// InitDemo creates a project with demo request specs. Exported for sidecar use.
func InitDemo(root string, stdout io.Writer) error {
	if err := os.Chdir(root); err != nil {
		return err
	}
	return runInit([]string{"--demo"}, stdout)
}

func rel(root, target string) string {
	value, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return value
}
