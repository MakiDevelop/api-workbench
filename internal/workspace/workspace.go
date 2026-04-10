package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/curlimport"
	"github.com/MakiDevelop/api-workbench/internal/diff"
	"github.com/MakiDevelop/api-workbench/internal/discover"
	"github.com/MakiDevelop/api-workbench/internal/envfile"
	"github.com/MakiDevelop/api-workbench/internal/history"
	"github.com/MakiDevelop/api-workbench/internal/openapiimport"
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
	Auth       *request.Auth       `json:"auth,omitempty"`
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
	// Resolve symlinks so paths are consistent with resolvePath output.
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	if collection == "" {
		collection = "requests"
	}

	envs, err := discover.EnvNames(root)
	if err != nil {
		return Info{}, err
	}

	collectionAbs, err := resolvePath(root, collection)
	if err != nil {
		return Info{}, err
	}

	requestFiles, err := discover.RequestFiles(collectionAbs)
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
			entry.Auth = spec.Auth
			entry.Assertions = spec.Assertions
			entry.Body = previewBody(spec.Body)
		}
		requests = append(requests, entry)
	}

	return Info{
		Root:           root,
		CollectionPath: discover.DisplayRelative(root, collectionAbs),
		Envs:           envs,
		Requests:       requests,
	}, nil
}

func RunSingle(requestPath string, options RunOptions) (RequestRun, error) {
	ctx, err := prepareContext(options)
	if err != nil {
		return RequestRun{}, err
	}

	absRequest, err := resolvePath(ctx.root, requestPath)
	if err != nil {
		return RequestRun{ExitCode: 1, RequestPath: requestPath, Error: err.Error()}, nil
	}
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
		recordHistory(ctx.root, ctx.envName, spec, result, response)
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

	recordHistory(ctx.root, ctx.envName, spec, result, response)
	return response, nil
}

func recordHistory(root, envName string, spec request.Spec, result runner.Result, run RequestRun) {
	// Best-effort: don't fail the run if history write fails.
	_ = history.Append(root, history.Entry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		RequestPath: run.RequestPath,
		RequestName: spec.Name,
		Env:         envName,
		Method:      result.Method,
		URL:         result.URL,
		StatusCode:  result.StatusCode,
		DurationMs:  result.DurationMS,
		ExitCode:    run.ExitCode,
		Error:       run.Error,
	})
}

func RunAll(collectionPath string, options RunOptions) (CollectionRun, error) {
	ctx, err := prepareContext(options)
	if err != nil {
		return CollectionRun{}, err
	}

	if collectionPath == "" {
		collectionPath = "requests"
	}

	collectionAbs, err := resolvePath(ctx.root, collectionPath)
	if err != nil {
		return CollectionRun{}, err
	}
	requestFiles, err := discover.RequestFiles(collectionAbs)
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
		absRequest, resolveErr := resolvePath(ctx.root, requestFile)
		if resolveErr != nil {
			run := RequestRun{ExitCode: 1, RequestPath: requestFile, Error: resolveErr.Error()}
			response.Runs = append(response.Runs, run)
			response.Summary.Total++
			response.Summary.Invalid++
			continue
		}
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

			recordHistory(ctx.root, ctx.envName, spec, result, run)
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
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
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

// SaveRequest writes a request.Spec as a formatted JSON file under the workspace.
// The filePath must be relative to the workspace root (e.g. "requests/my-api.json").
func SaveRequest(root, filePath string, spec request.Spec) (string, error) {
	wsRoot, err := findWorkspaceRoot(root)
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(wsRoot); resolveErr == nil {
		wsRoot = resolved
	}

	absPath, err := resolvePath(wsRoot, filePath)
	if err != nil {
		return "", err
	}

	if err := spec.Validate(); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return "", err
	}

	return discover.DisplayRelative(wsRoot, absPath), nil
}

// ImportCurl parses a cURL command and saves the resulting request spec.
// Returns the saved file path (relative to workspace root) and the parsed spec.
func ImportCurl(root, curlCmd, collection string) (string, request.Spec, error) {
	wsRoot, err := findWorkspaceRoot(root)
	if err != nil {
		return "", request.Spec{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(wsRoot); resolveErr == nil {
		wsRoot = resolved
	}

	spec, err := curlimport.Parse(curlCmd)
	if err != nil {
		return "", request.Spec{}, fmt.Errorf("curl parse: %w", err)
	}

	if collection == "" {
		collection = "requests"
	}

	// Generate a unique filename.
	baseName := sanitizeFilename(spec.Name)
	if baseName == "" {
		baseName = "imported"
	}
	fileName := baseName + ".json"
	filePath := filepath.Join(collection, fileName)

	// Check for conflicts and add a numeric suffix if needed.
	absPath := filepath.Join(wsRoot, filePath)
	for counter := 2; fileExists(absPath); counter++ {
		fileName = fmt.Sprintf("%s-%d.json", baseName, counter)
		filePath = filepath.Join(collection, fileName)
		absPath = filepath.Join(wsRoot, filePath)
	}

	saved, err := SaveRequest(wsRoot, filePath, spec)
	if err != nil {
		return "", request.Spec{}, err
	}

	return saved, spec, nil
}

// ImportOpenAPI parses an OpenAPI 3 JSON document and writes each operation
// as a request spec in the workspace. Returns the list of saved file paths.
func ImportOpenAPI(root string, data []byte, collection string) ([]string, error) {
	wsRoot, err := findWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(wsRoot); resolveErr == nil {
		wsRoot = resolved
	}

	specs, err := openapiimport.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("openapi parse: %w", err)
	}

	if collection == "" {
		collection = "requests"
	}

	var savedPaths []string
	for _, spec := range specs {
		baseName := sanitizeFilename(spec.Name)
		if baseName == "" {
			baseName = "imported"
		}
		fileName := baseName + ".json"
		filePath := filepath.Join(collection, fileName)

		// Avoid collisions.
		absPath := filepath.Join(wsRoot, filePath)
		for counter := 2; fileExists(absPath); counter++ {
			fileName = fmt.Sprintf("%s-%d.json", baseName, counter)
			filePath = filepath.Join(collection, fileName)
			absPath = filepath.Join(wsRoot, filePath)
		}

		saved, err := SaveRequest(wsRoot, filePath, spec)
		if err != nil {
			return savedPaths, fmt.Errorf("save %s: %w", spec.Name, err)
		}
		savedPaths = append(savedPaths, saved)
	}

	return savedPaths, nil
}

// SnapshotEntry represents a snapshot file in the workspace.
type SnapshotEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`       // relative to workspace root
	CapturedAt string `json:"capturedAt"` // from snapshot JSON
	IsLatest   bool   `json:"isLatest"`   // true for the non-timestamped current file
}

// ListSnapshots returns all snapshot files in the workspace, sorted newest first.
func ListSnapshots(root string) ([]SnapshotEntry, error) {
	wsRoot, err := findWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(wsRoot); resolveErr == nil {
		wsRoot = resolved
	}

	dir := filepath.Join(wsRoot, ".apiw", "snapshots")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var snapshots []SnapshotEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		absPath := filepath.Join(dir, e.Name())
		relPath := discover.DisplayRelative(wsRoot, absPath)

		// Read capturedAt from the file.
		capturedAt := ""
		if data, readErr := os.ReadFile(absPath); readErr == nil {
			var snap struct {
				CapturedAt string `json:"capturedAt"`
			}
			if json.Unmarshal(data, &snap) == nil {
				capturedAt = snap.CapturedAt
			}
		}

		// Determine if this is the latest (non-timestamped) snapshot.
		// Timestamped archives have 3 "--" separators, latest has 1.
		parts := strings.Split(strings.TrimSuffix(e.Name(), ".json"), "--")
		isLatest := len(parts) <= 2

		snapshots = append(snapshots, SnapshotEntry{
			Name:       e.Name(),
			Path:       relPath,
			CapturedAt: capturedAt,
			IsLatest:   isLatest,
		})
	}

	// Sort: newest first (by capturedAt descending).
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CapturedAt > snapshots[j].CapturedAt
	})

	return snapshots, nil
}

// ListHistory returns recent history entries across all daily files.
func ListHistory(root string, limit int) ([]history.Entry, error) {
	wsRoot, err := findWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(wsRoot); resolveErr == nil {
		wsRoot = resolved
	}
	return history.List(wsRoot, limit)
}

// DiffSnapshots compares two snapshot files and returns structured differences.
func DiffSnapshots(root, leftPath, rightPath string) (diff.SnapshotDiff, error) {
	wsRoot, err := findWorkspaceRoot(root)
	if err != nil {
		return diff.SnapshotDiff{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(wsRoot); resolveErr == nil {
		wsRoot = resolved
	}

	absLeft, err := resolvePath(wsRoot, leftPath)
	if err != nil {
		return diff.SnapshotDiff{}, fmt.Errorf("left path: %w", err)
	}
	absRight, err := resolvePath(wsRoot, rightPath)
	if err != nil {
		return diff.SnapshotDiff{}, fmt.Errorf("right path: %w", err)
	}

	leftData, err := os.ReadFile(absLeft)
	if err != nil {
		return diff.SnapshotDiff{}, fmt.Errorf("read left: %w", err)
	}
	rightData, err := os.ReadFile(absRight)
	if err != nil {
		return diff.SnapshotDiff{}, fmt.Errorf("read right: %w", err)
	}

	return diff.Snapshots(leftData, rightData)
}

func sanitizeFilename(name string) string {
	// Split camelCase / PascalCase into dash-separated words.
	// e.g. "listPets" -> "list-Pets", "HTTPServer" -> "HTTP-Server".
	var split strings.Builder
	runes := []rune(name)
	for i, ch := range runes {
		if i > 0 && ch >= 'A' && ch <= 'Z' {
			prev := runes[i-1]
			// Insert dash when transitioning from lower/digit to upper.
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				split.WriteRune('-')
			}
		}
		split.WriteRune(ch)
	}

	var b strings.Builder
	for _, ch := range strings.ToLower(split.String()) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			b.WriteRune(ch)
		} else if ch == ' ' || ch == '/' {
			b.WriteRune('-')
		}
	}
	result := b.String()
	// Collapse consecutive dashes.
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	result = strings.Trim(result, "-")
	if len(result) > 80 {
		result = result[:80]
	}
	return result
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var errPathTraversal = fmt.Errorf("path escapes workspace root")

func resolvePath(root, value string) (string, error) {
	var abs string
	if filepath.IsAbs(value) {
		abs = filepath.Clean(value)
	} else {
		abs = filepath.Join(root, value)
	}

	// Resolve symlinks to catch symlink-based traversal.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Path doesn't exist yet (e.g. snapshot dir) — fall back to lexical check.
		resolved = abs
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}

	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errPathTraversal
	}
	return resolved, nil
}

func classifyRunError(err error) int {
	var assertionErr *runner.AssertionError
	if errors.As(err, &assertionErr) {
		return 3
	}
	return 2
}
