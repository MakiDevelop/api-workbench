package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MakiDevelop/api-workbench/internal/workspace"
)

func runImport(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("import requires a subcommand: curl | openapi")
	}

	switch args[0] {
	case "curl":
		return runImportCurl(args[1:], stdout)
	case "openapi":
		return runImportOpenAPI(args[1:], stdout)
	default:
		return fmt.Errorf("unknown import format: %s (supported: curl, openapi)", args[0])
	}
}

func runImportCurl(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("import curl requires a cURL command string")
	}

	curlCmd := strings.Join(args, " ")

	savedPath, spec, err := workspace.ImportCurl(".", curlCmd, "requests")
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "imported       %s\n", savedPath)
	fmt.Fprintf(stdout, "name           %s\n", spec.Name)
	fmt.Fprintf(stdout, "method         %s\n", spec.Method)
	fmt.Fprintf(stdout, "url            %s\n", spec.URL)

	return nil
}

func runImportOpenAPI(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("import openapi requires a file path")
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read %s: %w", args[0], err)
	}

	savedPaths, err := workspace.ImportOpenAPI(".", data, "requests")
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "imported       %d request(s) from %s\n", len(savedPaths), args[0])
	for _, p := range savedPaths {
		fmt.Fprintf(stdout, "  %s\n", p)
	}
	return nil
}
