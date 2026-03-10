package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/workspace"
)

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "expected command: workspace | run-request | run-collection")
		return 1
	}

	switch args[0] {
	case "workspace":
		return runWorkspace(args[1:], stdout, stderr)
	case "run-request":
		return runRequest(args[1:], stdout, stderr)
	case "run-collection":
		return runCollection(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
}

func runWorkspace(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("workspace", flag.ContinueOnError)
	fs.SetOutput(stderr)

	root := fs.String("root", ".", "workspace root")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	collection := "requests"
	if fs.NArg() > 0 {
		collection = fs.Arg(0)
	}

	info, err := workspace.LoadInfo(*root, collection)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	return writeJSON(stdout, info)
}

func runRequest(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("run-request", flag.ContinueOnError)
	fs.SetOutput(stderr)

	root := fs.String("root", ".", "workspace root")
	requestPath := fs.String("request", "", "request path")
	envName := fs.String("env", "local", "environment")
	timeoutMS := fs.Int("timeout-ms", 15000, "timeout in milliseconds")
	snapshot := fs.Bool("snapshot", false, "write snapshot")

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *requestPath == "" {
		fmt.Fprintln(stderr, "--request is required")
		return 1
	}

	result, err := workspace.RunSingle(*requestPath, workspace.RunOptions{
		Root:     *root,
		EnvName:  *envName,
		Timeout:  time.Duration(*timeoutMS) * time.Millisecond,
		Snapshot: *snapshot,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	return writeJSON(stdout, result)
}

func runCollection(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("run-collection", flag.ContinueOnError)
	fs.SetOutput(stderr)

	root := fs.String("root", ".", "workspace root")
	collectionPath := fs.String("collection", "requests", "collection path")
	envName := fs.String("env", "local", "environment")
	timeoutMS := fs.Int("timeout-ms", 15000, "timeout in milliseconds")
	snapshot := fs.Bool("snapshot", false, "write snapshot")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	result, err := workspace.RunAll(*collectionPath, workspace.RunOptions{
		Root:     *root,
		EnvName:  *envName,
		Timeout:  time.Duration(*timeoutMS) * time.Millisecond,
		Snapshot: *snapshot,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	return writeJSON(stdout, result)
}

func writeJSON(stdout *os.File, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
