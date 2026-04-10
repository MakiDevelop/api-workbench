package app

import (
	"fmt"
	"io"
)

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	case "init":
		if err := runInit(args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "init failed: %v\n", err)
			return 1
		}
		return 0
	case "run":
		code, err := runRequest(args[1:], stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "run failed: %v\n", err)
		}
		return code
	case "import":
		if err := runImport(args[1:], stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "import failed: %v\n", err)
			return 1
		}
		return 0
	case "tui":
		if err := runTUI(args[1:], stdin, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "tui failed: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "apiw - CLI-first API workbench")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  apiw init [--demo]")
	fmt.Fprintln(w, "  apiw run <request-file> [--env local] [--timeout 15s] [--snapshot]")
	fmt.Fprintln(w, "  apiw run --all [requests-dir] [--env local] [--timeout 15s] [--snapshot]")
	fmt.Fprintln(w, "  apiw import curl <curl-command>")
	fmt.Fprintln(w, "  apiw tui [requests-dir] [--env local] [--timeout 15s] [--snapshot]")
}
