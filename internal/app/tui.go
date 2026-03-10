package app

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/project"
)

type tuiState struct {
	root            string
	collectionPath  string
	envs            []string
	selectedEnv     int
	requests        []string
	selectedRequest int
	snapshot        bool
	timeout         time.Duration
	status          string
	lastOutput      string
}

func runTUI(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	collectionPath, flagArgs, err := splitSinglePathArgs(args, map[string]bool{
		"--env":     true,
		"--timeout": true,
	})
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(stderr)

	envName := fs.String("env", "local", "environment name")
	timeout := fs.Duration("timeout", 15*time.Second, "request timeout")
	snapshot := fs.Bool("snapshot", false, "write snapshot files while running")

	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	root, err := project.FindRoot(".")
	if err != nil {
		return err
	}

	if collectionPath == "" {
		collectionPath = "requests"
	}
	resolvedCollectionPath := collectionPath
	if !filepath.IsAbs(resolvedCollectionPath) {
		resolvedCollectionPath = filepath.Join(root, collectionPath)
	}

	state, err := newTUIState(root, resolvedCollectionPath, *envName, *timeout, *snapshot)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(stdin)
	clear := shouldUseANSI(stdout)

	for {
		renderTUI(stdout, state, clear)

		line, readErr := reader.ReadString('\n')
		if readErr != nil && len(line) == 0 {
			if readErr == io.EOF {
				fmt.Fprintln(stdout)
				return nil
			}
			return readErr
		}

		command := strings.TrimSpace(line)
		quit, commandErr := handleTUICommand(&state, command)
		if commandErr != nil {
			state.status = "error: " + commandErr.Error()
		}
		if quit {
			fmt.Fprintln(stdout)
			return nil
		}

		if readErr == io.EOF {
			fmt.Fprintln(stdout)
			return nil
		}
	}
}

func newTUIState(root, collectionPath, envName string, timeout time.Duration, snapshot bool) (tuiState, error) {
	envs, err := discoverEnvNames(root)
	if err != nil {
		return tuiState{}, err
	}
	if len(envs) == 0 {
		return tuiState{}, fmt.Errorf("no env files found under %s", filepath.Join(root, ".apiw", "env"))
	}

	requests, err := discoverRequestFiles(collectionPath)
	if err != nil {
		return tuiState{}, err
	}
	if len(requests) == 0 {
		return tuiState{}, fmt.Errorf("no request specs found under %s", collectionPath)
	}

	selectedEnv, err := selectNameOrIndex(envs, envName)
	if err != nil {
		selectedEnv = 0
	}

	return tuiState{
		root:            root,
		collectionPath:  collectionPath,
		envs:            envs,
		selectedEnv:     selectedEnv,
		requests:        requests,
		selectedRequest: 0,
		snapshot:        snapshot,
		timeout:         timeout,
		status:          "ready",
	}, nil
}

func handleTUICommand(state *tuiState, raw string) (bool, error) {
	command := strings.TrimSpace(raw)
	if command == "" {
		state.status = "ready"
		return false, nil
	}

	fields := strings.Fields(command)
	switch fields[0] {
	case "q", "quit", "exit":
		return true, nil
	case "h", "help":
		state.status = "commands: [number] select | r run | a run all | e <env> switch env | s toggle snapshot | reload | q quit"
		return false, nil
	case "r", "run":
		return false, executeTUIRequest(state)
	case "a", "all":
		return false, executeTUICollection(state)
	case "s", "snapshot":
		state.snapshot = !state.snapshot
		if state.snapshot {
			state.status = "snapshot writing enabled"
		} else {
			state.status = "snapshot writing disabled"
		}
		return false, nil
	case "reload":
		return false, reloadTUIState(state)
	case "e", "env":
		if len(fields) < 2 {
			return false, fmt.Errorf("env command requires a name or index")
		}
		index, err := selectNameOrIndex(state.envs, fields[1])
		if err != nil {
			return false, err
		}
		state.selectedEnv = index
		state.status = fmt.Sprintf("selected env %s", state.currentEnv())
		return false, nil
	}

	index, err := selectNameOrIndex(tuiDisplayPaths(state.root, state.requests), fields[0])
	if err == nil {
		state.selectedRequest = index
		state.status = fmt.Sprintf("selected request %s", state.displayRequest(state.requests[index]))
		return false, nil
	}

	return false, fmt.Errorf("unknown command: %s", command)
}

func executeTUIRequest(state *tuiState) error {
	ctx, err := prepareRunContext(state.currentEnv(), state.timeout)
	if err != nil {
		return err
	}

	var output bytes.Buffer
	code, runErr := runRequestFile(state.requests[state.selectedRequest], ctx, state.snapshot, &output)
	state.lastOutput = trimOutput(output.String(), 18)
	state.status = summarizeRunStatus(code, state.displayRequest(state.requests[state.selectedRequest]), state.currentEnv(), state.snapshot)
	return runErr
}

func executeTUICollection(state *tuiState) error {
	ctx, err := prepareRunContext(state.currentEnv(), state.timeout)
	if err != nil {
		return err
	}

	var output bytes.Buffer
	var errorOutput bytes.Buffer
	code, runErr := runCollectionFiles(state.requests, ctx, state.snapshot, state.displayRequest, &output, &errorOutput)
	combined := strings.TrimSpace(output.String())
	if errorOutput.Len() > 0 {
		if combined != "" {
			combined += "\n"
		}
		combined += strings.TrimSpace(errorOutput.String())
	}
	state.lastOutput = trimOutput(combined, 24)
	state.status = summarizeRunStatus(code, fmt.Sprintf("%d requests", len(state.requests)), state.currentEnv(), state.snapshot)
	return runErr
}

func reloadTUIState(state *tuiState) error {
	currentEnv := state.currentEnv()
	currentRequest := state.requests[state.selectedRequest]

	envs, err := discoverEnvNames(state.root)
	if err != nil {
		return err
	}
	requests, err := discoverRequestFiles(state.collectionPath)
	if err != nil {
		return err
	}
	if len(requests) == 0 {
		return fmt.Errorf("no request specs found under %s", state.collectionPath)
	}

	state.envs = envs
	if index, err := selectNameOrIndex(envs, currentEnv); err == nil {
		state.selectedEnv = index
	} else {
		state.selectedEnv = 0
	}

	state.requests = requests
	state.selectedRequest = selectRequestIndex(requests, currentRequest)
	state.status = "reloaded envs and requests"
	return nil
}

func discoverEnvNames(root string) ([]string, error) {
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

func splitSinglePathArgs(args []string, valueFlags map[string]bool) (string, []string, error) {
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
			return "", nil, fmt.Errorf("command accepts only one path argument")
		}
		positional = arg
	}

	if waitingForValue != "" {
		return "", nil, fmt.Errorf("missing value for %s", waitingForValue)
	}

	return positional, flags, nil
}

func renderTUI(stdout io.Writer, state tuiState, clear bool) {
	if clear {
		fmt.Fprint(stdout, "\x1b[2J\x1b[H")
	}

	fmt.Fprintln(stdout, "apiw tui")
	fmt.Fprintln(stdout, "")
	fmt.Fprintf(stdout, "project        %s\n", state.root)
	fmt.Fprintf(stdout, "collection     %s\n", displayRelative(state.root, state.collectionPath))
	fmt.Fprintf(stdout, "env            %s\n", state.currentEnv())
	fmt.Fprintf(stdout, "snapshot       %t\n", state.snapshot)
	fmt.Fprintf(stdout, "timeout        %s\n", state.timeout)
	fmt.Fprintf(stdout, "requests       %d\n", len(state.requests))
	fmt.Fprintln(stdout, "")

	fmt.Fprintln(stdout, "envs")
	for index, env := range state.envs {
		marker := " "
		if index == state.selectedEnv {
			marker = "*"
		}
		fmt.Fprintf(stdout, "  %s %d. %s\n", marker, index+1, env)
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "requests")
	for index, path := range state.requests {
		marker := " "
		if index == state.selectedRequest {
			marker = ">"
		}
		fmt.Fprintf(stdout, "  %s %d. %s\n", marker, index+1, state.displayRequest(path))
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "commands       [number] select | r run | a run all | e <env> switch env | s toggle snapshot | reload | q quit")
	fmt.Fprintf(stdout, "status         %s\n", state.status)

	if state.lastOutput != "" {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, "last output")
		fmt.Fprintln(stdout, indentBlock(state.lastOutput, "  "))
	}

	fmt.Fprintln(stdout, "")
	fmt.Fprint(stdout, "apiw> ")
}

func (state tuiState) currentEnv() string {
	return state.envs[state.selectedEnv]
}

func (state tuiState) displayRequest(path string) string {
	return displayRelative(state.root, path)
}

func selectNameOrIndex(values []string, query string) (int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0, fmt.Errorf("selection cannot be empty")
	}

	if index, err := strconv.Atoi(query); err == nil {
		if index < 1 || index > len(values) {
			return 0, fmt.Errorf("selection %d out of range", index)
		}
		return index - 1, nil
	}

	for index, value := range values {
		if value == query {
			return index, nil
		}
	}

	return 0, fmt.Errorf("unknown selection: %s", query)
}

func selectRequestIndex(requests []string, previous string) int {
	for index, request := range requests {
		if request == previous {
			return index
		}
	}
	return 0
}

func summarizeRunStatus(code int, target, envName string, snapshot bool) string {
	status := "passed"
	switch code {
	case 1:
		status = "invalid"
	case 2:
		status = "transport error"
	case 3:
		status = "assertion failed"
	}

	snapshotLabel := "snapshot off"
	if snapshot {
		snapshotLabel = "snapshot on"
	}

	return fmt.Sprintf("%s: %s (%s, %s)", status, target, envName, snapshotLabel)
}

func trimOutput(value string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}

	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func indentBlock(value, indent string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		lines[index] = indent + line
	}
	return strings.Join(lines, "\n")
}

func displayRelative(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return relative
}

func tuiDisplayPaths(root string, paths []string) []string {
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		values = append(values, displayRelative(root, path))
	}
	return values
}

func shouldUseANSI(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return os.Getenv("TERM") != "dumb"
}
