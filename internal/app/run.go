package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/envfile"
	"github.com/MakiDevelop/api-workbench/internal/project"
	"github.com/MakiDevelop/api-workbench/internal/request"
	"github.com/MakiDevelop/api-workbench/internal/runner"
)

func runRequest(args []string, stdout, stderr io.Writer) (int, error) {
	reqPath, flagArgs, err := splitRunArgs(args)
	if err != nil {
		return 1, err
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	envName := fs.String("env", "local", "environment name")
	timeout := fs.Duration("timeout", 15*time.Second, "request timeout")
	snapshot := fs.Bool("snapshot", false, "write snapshot")
	runAll := fs.Bool("all", false, "run all request specs in a directory")

	if err := fs.Parse(flagArgs); err != nil {
		return 1, err
	}

	if *runAll {
		collectionPath := reqPath
		if collectionPath == "" {
			collectionPath = "requests"
		}

		return runCollection(collectionPath, *envName, *timeout, *snapshot, stdout, stderr)
	}

	if reqPath == "" {
		return 1, fmt.Errorf("run requires exactly one request file")
	}

	ctx, err := prepareRunContext(*envName, *timeout)
	if err != nil {
		return 1, err
	}

	return runRequestFile(reqPath, ctx, *snapshot, stdout)
}

func splitRunArgs(args []string) (string, []string, error) {
	valueFlags := map[string]bool{
		"--env":     true,
		"--timeout": true,
	}

	var positional string
	var flags []string
	var waitingForValue string

	for _, arg := range args {
		if waitingForValue != "" {
			flags = append(flags, arg)
			waitingForValue = ""
			continue
		}

		if valueFlags[arg] {
			flags = append(flags, arg)
			waitingForValue = arg
			continue
		}

		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			continue
		}

		if positional != "" {
			return "", nil, fmt.Errorf("run accepts only one path argument")
		}
		positional = arg
	}

	if waitingForValue != "" {
		return "", nil, fmt.Errorf("missing value for %s", waitingForValue)
	}

	return positional, flags, nil
}

type runContext struct {
	root    string
	envName string
	opts    runner.Options
}

func prepareRunContext(envName string, timeout time.Duration) (runContext, error) {
	root, err := project.FindRoot(".")
	if err != nil {
		return runContext{}, err
	}

	vars, err := envfile.Load(filepath.Join(root, ".apiw", "env", envName+".env"))
	if err != nil {
		return runContext{}, err
	}

	return runContext{
		root:    root,
		envName: envName,
		opts: runner.Options{
			Variables: vars,
			Timeout:   timeout,
		},
	}, nil
}

func runRequestFile(reqPath string, ctx runContext, snapshot bool, stdout io.Writer) (int, error) {
	spec, err := request.Load(reqPath)
	if err != nil {
		return 1, err
	}

	result, runErr := runner.Run(spec, ctx.opts)
	if runErr != nil {
		var assertionErr *runner.AssertionError
		if ok := asAssertionError(runErr, &assertionErr); ok {
			printResult(stdout, result)
			return 3, runErr
		}
		return 2, runErr
	}

	printResult(stdout, result)

	if snapshot {
		path, err := runner.WriteSnapshot(ctx.root, ctx.envName, spec, result)
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(stdout, "snapshot       %s\n", path)
	}

	return 0, nil
}

func runCollection(collectionPath, envName string, timeout time.Duration, snapshot bool, stdout, stderr io.Writer) (int, error) {
	ctx, err := prepareRunContext(envName, timeout)
	if err != nil {
		return 1, err
	}

	files, err := discoverRequestFiles(collectionPath)
	if err != nil {
		return 1, err
	}
	if len(files) == 0 {
		return 1, fmt.Errorf("no request specs found under %s", collectionPath)
	}

	var passed int
	var failed int
	var transport int
	var invalid int

	for index, path := range files {
		if index > 0 {
			fmt.Fprintln(stdout, "")
		}

		fmt.Fprintf(stdout, "file           %s\n", path)
		code, runErr := runRequestFile(path, ctx, snapshot, stdout)
		if runErr != nil {
			fmt.Fprintf(stderr, "error          %s: %v\n", path, runErr)
		}

		switch code {
		case 0:
			passed++
		case 1:
			invalid++
		case 2:
			transport++
		case 3:
			failed++
		}
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "summary        total=%d passed=%d failed=%d transport=%d invalid=%d\n", len(files), passed, failed, transport, invalid)

	switch {
	case invalid > 0:
		return 1, fmt.Errorf("collection completed with invalid request specs")
	case transport > 0:
		return 2, fmt.Errorf("collection completed with transport errors")
	case failed > 0:
		return 3, fmt.Errorf("collection completed with assertion failures")
	default:
		return 0, nil
	}
}

func discoverRequestFiles(collectionPath string) ([]string, error) {
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

func printResult(stdout io.Writer, result runner.Result) {
	fmt.Fprintf(stdout, "request        %s %s\n", result.Method, result.URL)
	fmt.Fprintf(stdout, "status         %d\n", result.StatusCode)
	fmt.Fprintf(stdout, "duration       %s\n", result.Duration.Round(time.Millisecond))

	if len(result.AssertionMessages) > 0 {
		for _, message := range result.AssertionMessages {
			fmt.Fprintf(stdout, "assertion      %s\n", message)
		}
	}

	body := strings.TrimSpace(result.Body)
	if body == "" {
		return
	}

	fmt.Fprintln(stdout, "body")
	fmt.Fprintln(stdout, indentBody(body))
}

func indentBody(body string) string {
	var payload any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return "  " + body
	}

	formatted, err := json.MarshalIndent(payload, "  ", "  ")
	if err != nil {
		return "  " + body
	}

	lines := strings.Split(string(formatted), "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func asAssertionError(err error, target **runner.AssertionError) bool {
	value, ok := err.(*runner.AssertionError)
	if !ok {
		return false
	}
	*target = value
	return true
}
