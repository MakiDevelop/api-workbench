package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/discover"
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

	envs, err := discover.EnvNames(root)
	if err != nil {
		return Info{}, err
	}

	requestFiles, err := discover.RequestFiles(resolvePath(root, collection))
	if err != nil {
		return Info{}, err
	}

	requests := make([]RequestEntry, 0, len(requestFiles))
	for _, path := range requestFiles {
		entry := RequestEntry{
			Path: discover.DisplayRelative(root, path),
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
		CollectionPath: discover.DisplayRelative(root, resolvePath(root, collection)),
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
	displayRequest := discover.DisplayRelative(ctx.root, absRequest)
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
		response.SnapshotPath = discover.DisplayRelative(ctx.root, snapshotPath)
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

	requestFiles, err := discover.RequestFiles(resolvePath(ctx.root, collectionPath))
	if err != nil {
		return CollectionRun{}, err
	}
	if len(requestFiles) == 0 {
		return CollectionRun{}, fmt.Errorf("no request specs found under %s", collectionPath)
	}

	// Shared client for cookie persistence across the collection run.
	sharedClient := runner.NewSharedClient(ctx.runnerOptions.Timeout)
	ctx.runnerOptions.Client = sharedClient

	response := CollectionRun{
		Environ: ctx.envName,
		Runs:    make([]RequestRun, 0, len(requestFiles)),
	}

	for _, requestFile := range requestFiles {
		absRequest := resolvePath(ctx.root, requestFile)
		displayRequest := discover.DisplayRelative(ctx.root, absRequest)
		spec, loadErr := request.Load(absRequest)

		var run RequestRun
		if loadErr != nil {
			run = RequestRun{
				ExitCode:    1,
				RequestPath: displayRequest,
				Error:       loadErr.Error(),
			}
		} else {
			result, runErr := runner.Run(spec, ctx.runnerOptions)
			run = RequestRun{
				RequestName: spec.Name,
				RequestPath: displayRequest,
				Result:      &result,
			}
			if runErr != nil {
				run.ExitCode = classifyRunError(runErr)
				run.Error = runErr.Error()
			}

			// Merge extracted values into variables for subsequent requests (chaining).
			for k, v := range result.Extracted {
				ctx.runnerOptions.Variables[k] = v
			}

			if options.Snapshot && run.ExitCode == 0 {
				snapshotPath, snapshotErr := runner.WriteSnapshot(ctx.root, ctx.envName, spec, result)
				if snapshotErr != nil {
					return CollectionRun{}, snapshotErr
				}
				run.SnapshotPath = discover.DisplayRelative(ctx.root, snapshotPath)
			}
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

func resolvePath(root, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}

func classifyRunError(err error) int {
	var assertionErr *runner.AssertionError
	if errors.As(err, &assertionErr) {
		return 3
	}
	return 2
}
