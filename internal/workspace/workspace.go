package workspace

import (
	"bytes"
	"encoding/json"
	"fmt"
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

type Info struct {
	Root           string         `json:"root"`
	CollectionPath string         `json:"collectionPath"`
	Envs           []string       `json:"envs"`
	Requests       []RequestEntry `json:"requests"`
}

type RequestEntry struct {
	Name       string              `json:"name"`
	Path       string              `json:"path"`
	Method     string              `json:"method,omitempty"`
	URL        string              `json:"url,omitempty"`
	Headers    map[string]string   `json:"headers,omitempty"`
	Query      map[string]string   `json:"query,omitempty"`
	Body       *RequestBodyPreview `json:"body,omitempty"`
	Assertions []request.Assertion `json:"assertions,omitempty"`
	LoadError  string              `json:"loadError,omitempty"`
}

type RequestBodyPreview struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type RequestRun struct {
	ExitCode     int            `json:"exitCode"`
	RequestName  string         `json:"requestName"`
	RequestPath  string         `json:"requestPath"`
	SnapshotPath string         `json:"snapshotPath,omitempty"`
	Error        string         `json:"error,omitempty"`
	Result       *runner.Result `json:"result,omitempty"`
}

type CollectionRun struct {
	ExitCode int             `json:"exitCode"`
	Environ  string          `json:"env"`
	Runs     []RequestRun    `json:"runs"`
	Summary  CollectionStats `json:"summary"`
	Error    string          `json:"error,omitempty"`
}

type CollectionStats struct {
	Total     int `json:"total"`
	Passed    int `json:"passed"`
	Failed    int `json:"failed"`
	Transport int `json:"transport"`
	Invalid   int `json:"invalid"`
}

type RunOptions struct {
	Root       string
	EnvName    string
	Timeout    time.Duration
	Snapshot   bool
	Collection string
}

func LoadInfo(startRoot, collection string) (Info, error) {
	root, err := findWorkspaceRoot(startRoot)
	if err != nil {
		return Info{}, err
	}
	if collection == "" {
		collection = "requests"
	}

	envs, err := discoverEnvNames(root)
	if err != nil {
		return Info{}, err
	}

	requestFiles, err := discoverRequestFiles(resolvePath(root, collection))
	if err != nil {
		return Info{}, err
	}

	requests := make([]RequestEntry, 0, len(requestFiles))
	for _, path := range requestFiles {
		entry := RequestEntry{
			Path: displayRelative(root, path),
		}

		spec, loadErr := request.Load(path)
		if loadErr != nil {
			entry.Name = filepath.Base(path)
			entry.LoadError = loadErr.Error()
		} else {
			entry.Name = spec.Name
			entry.Method = strings.ToUpper(spec.Method)
			entry.URL = spec.URL
			entry.Headers = spec.Headers
			entry.Query = spec.Query
			entry.Assertions = spec.Assertions
			entry.Body = previewBody(spec.Body)
		}
		requests = append(requests, entry)
	}

	return Info{
		Root:           root,
		CollectionPath: displayRelative(root, resolvePath(root, collection)),
		Envs:           envs,
		Requests:       requests,
	}, nil
}

func RunSingle(requestPath string, options RunOptions) (RequestRun, error) {
	ctx, err := prepareContext(options)
	if err != nil {
		return RequestRun{}, err
	}

	absRequest := resolvePath(ctx.root, requestPath)
	displayRequest := displayRelative(ctx.root, absRequest)
	spec, err := request.Load(absRequest)
	if err != nil {
		return RequestRun{
			ExitCode:    1,
			RequestPath: displayRequest,
			Error:       err.Error(),
		}, nil
	}

	result, runErr := runner.Run(spec, ctx.runnerOptions)
	response := RequestRun{
		RequestName: spec.Name,
		RequestPath: displayRequest,
		Result:      &result,
	}

	if runErr != nil {
		response.ExitCode = classifyRunError(runErr)
		response.Error = runErr.Error()
		return response, nil
	}

	response.ExitCode = 0

	if options.Snapshot {
		snapshotPath, snapshotErr := runner.WriteSnapshot(ctx.root, ctx.envName, spec, result)
		if snapshotErr != nil {
			return RequestRun{}, snapshotErr
		}
		response.SnapshotPath = displayRelative(ctx.root, snapshotPath)
	}

	return response, nil
}

func RunAll(collectionPath string, options RunOptions) (CollectionRun, error) {
	ctx, err := prepareContext(options)
	if err != nil {
		return CollectionRun{}, err
	}

	if collectionPath == "" {
		collectionPath = "requests"
	}

	requestFiles, err := discoverRequestFiles(resolvePath(ctx.root, collectionPath))
	if err != nil {
		return CollectionRun{}, err
	}
	if len(requestFiles) == 0 {
		return CollectionRun{}, fmt.Errorf("no request specs found under %s", collectionPath)
	}

	response := CollectionRun{
		Environ: ctx.envName,
		Runs:    make([]RequestRun, 0, len(requestFiles)),
	}

	for _, requestFile := range requestFiles {
		run, runErr := RunSingle(requestFile, RunOptions{
			Root:     ctx.root,
			EnvName:  ctx.envName,
			Timeout:  ctx.runnerOptions.Timeout,
			Snapshot: options.Snapshot,
		})
		if runErr != nil {
			return CollectionRun{}, runErr
		}

		response.Runs = append(response.Runs, run)
		response.Summary.Total++

		switch run.ExitCode {
		case 0:
			response.Summary.Passed++
		case 1:
			response.Summary.Invalid++
		case 2:
			response.Summary.Transport++
		case 3:
			response.Summary.Failed++
		}
	}

	switch {
	case response.Summary.Invalid > 0:
		response.ExitCode = 1
		response.Error = "collection completed with invalid request specs"
	case response.Summary.Transport > 0:
		response.ExitCode = 2
		response.Error = "collection completed with transport errors"
	case response.Summary.Failed > 0:
		response.ExitCode = 3
		response.Error = "collection completed with assertion failures"
	default:
		response.ExitCode = 0
	}

	return response, nil
}

type runContext struct {
	root          string
	envName       string
	runnerOptions runner.Options
}

func prepareContext(options RunOptions) (runContext, error) {
	root, err := findWorkspaceRoot(options.Root)
	if err != nil {
		return runContext{}, err
	}

	envName := options.EnvName
	if envName == "" {
		envName = "local"
	}

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	vars, err := envfile.Load(filepath.Join(root, ".apiw", "env", envName+".env"))
	if err != nil {
		return runContext{}, err
	}

	return runContext{
		root:    root,
		envName: envName,
		runnerOptions: runner.Options{
			Variables: vars,
			Timeout:   timeout,
		},
	}, nil
}

func findWorkspaceRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}
	return project.FindRoot(start)
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

func previewBody(body *request.Body) *RequestBodyPreview {
	if body == nil {
		return nil
	}

	preview := &RequestBodyPreview{
		Type:    strings.TrimSpace(body.Type),
		Content: strings.TrimSpace(string(body.Content)),
	}

	switch strings.ToLower(preview.Type) {
	case "", "json":
		var formatted bytes.Buffer
		if len(body.Content) > 0 && json.Indent(&formatted, body.Content, "", "  ") == nil {
			preview.Content = formatted.String()
		}
	case "text":
		var value string
		if err := json.Unmarshal(body.Content, &value); err == nil {
			preview.Content = value
		}
	}

	return preview
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

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func displayRelative(root, value string) string {
	relative, err := filepath.Rel(root, value)
	if err != nil {
		return value
	}
	return relative
}

func classifyRunError(err error) int {
	var assertionErr *runner.AssertionError
	if ok := asAssertionError(err, &assertionErr); ok {
		return 3
	}
	return 2
}

func asAssertionError(err error, target **runner.AssertionError) bool {
	value, ok := err.(*runner.AssertionError)
	if !ok {
		return false
	}
	*target = value
	return true
}
