package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/envfile"
	"github.com/MakiDevelop/api-workbench/internal/project"
	"github.com/MakiDevelop/api-workbench/internal/request"
	"github.com/MakiDevelop/api-workbench/internal/runner"
)

func runRequest(args []string, stdout, stderr io.Writer) (int, error) {
	var reqPath string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		reqPath = args[0]
		args = args[1:]
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	envName := fs.String("env", "local", "environment name")
	timeout := fs.Duration("timeout", 15*time.Second, "request timeout")
	snapshot := fs.Bool("snapshot", false, "write snapshot")

	if err := fs.Parse(args); err != nil {
		return 1, err
	}

	if reqPath != "" && fs.NArg() > 0 {
		return 1, fmt.Errorf("run accepts only one request file")
	}

	if reqPath == "" && fs.NArg() != 1 {
		return 1, fmt.Errorf("run requires exactly one request file")
	}

	if reqPath == "" {
		reqPath = fs.Arg(0)
	}

	spec, err := request.Load(reqPath)
	if err != nil {
		return 1, err
	}

	root, err := project.FindRoot(".")
	if err != nil {
		return 1, err
	}

	vars, err := envfile.Load(filepath.Join(root, ".apiw", "env", *envName+".env"))
	if err != nil {
		return 1, err
	}

	opts := runner.Options{
		Variables: vars,
		Timeout:   *timeout,
	}

	result, runErr := runner.Run(spec, opts)
	if runErr != nil {
		var assertionErr *runner.AssertionError
		if ok := asAssertionError(runErr, &assertionErr); ok {
			printResult(stdout, result)
			return 3, runErr
		}
		return 2, runErr
	}

	printResult(stdout, result)

	if *snapshot {
		path, err := runner.WriteSnapshot(root, *envName, spec, result)
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(stdout, "snapshot       %s\n", path)
	}

	return 0, nil
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
