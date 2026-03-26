package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/MakiDevelop/api-workbench/internal/app"
	"github.com/MakiDevelop/api-workbench/internal/workspace"
)

const protocolVersion = "1"

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "expected command: serve | workspace | run-request | run-collection")
		return 1
	}

	switch args[0] {
	case "serve":
		return runServe(os.Stdin, stdout, stderr)
	case "workspace":
		return runWorkspace(args[1:], stdout, stderr)
	case "run-request":
		return runRequest(args[1:], stdout, stderr)
	case "run-collection":
		return runCollection(args[1:], stdout, stderr)
	case "init-demo":
		return runInitDemo(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 1
	}
}

// --- JSON-RPC types ---

type rpcRequest struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcResponse struct {
	ID      uint64 `json:"id"`
	Version string `json:"version"`
	Result  any    `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

// --- Param types ---

type workspaceParams struct {
	Root       string `json:"root"`
	Collection string `json:"collection"`
}

type runRequestParams struct {
	Root        string `json:"root"`
	RequestPath string `json:"requestPath"`
	EnvName     string `json:"envName"`
	TimeoutMs   int    `json:"timeoutMs"`
	Snapshot    bool   `json:"snapshot"`
}

type runCollectionParams struct {
	Root           string `json:"root"`
	CollectionPath string `json:"collectionPath"`
	EnvName        string `json:"envName"`
	TimeoutMs      int    `json:"timeoutMs"`
	Snapshot       bool   `json:"snapshot"`
}

// --- Serve mode: persistent JSON-RPC on stdin/stdout ---

func runServe(stdin io.Reader, stdout, stderr *os.File) int {
	scanner := bufio.NewScanner(stdin)
	// Allow up to 10 MB per line (large JSON responses in results).
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	encoder := json.NewEncoder(stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			resp := rpcResponse{ID: 0, Version: protocolVersion, Error: "invalid JSON: " + err.Error()}
			encoder.Encode(resp)
			continue
		}

		resp := handleRPC(req)
		if err := encoder.Encode(resp); err != nil {
			fmt.Fprintln(stderr, "encode error:", err)
		}

		if req.Method == "shutdown" {
			return 0
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(stderr, "stdin read error:", err)
		return 1
	}

	return 0
}

func handleRPC(req rpcRequest) rpcResponse {
	base := rpcResponse{ID: req.ID, Version: protocolVersion}

	switch req.Method {
	case "ping":
		base.Result = map[string]string{"status": "pong"}
		return base

	case "version":
		base.Result = map[string]string{"protocol": protocolVersion}
		return base

	case "workspace":
		var p workspaceParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			base.Error = "invalid params: " + err.Error()
			return base
		}
		if p.Collection == "" {
			p.Collection = "requests"
		}
		info, err := workspace.LoadInfo(p.Root, p.Collection)
		if err != nil {
			base.Error = err.Error()
			return base
		}
		base.Result = info
		return base

	case "run_request":
		var p runRequestParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			base.Error = "invalid params: " + err.Error()
			return base
		}
		timeoutMs := p.TimeoutMs
		if timeoutMs <= 0 {
			timeoutMs = 15000
		}
		result, err := workspace.RunSingle(p.RequestPath, workspace.RunOptions{
			Root:    p.Root,
			EnvName: p.EnvName,
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
			Snapshot: p.Snapshot,
		})
		if err != nil {
			base.Error = err.Error()
			return base
		}
		base.Result = result
		return base

	case "run_collection":
		var p runCollectionParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			base.Error = "invalid params: " + err.Error()
			return base
		}
		timeoutMs := p.TimeoutMs
		if timeoutMs <= 0 {
			timeoutMs = 15000
		}
		collectionPath := p.CollectionPath
		if collectionPath == "" {
			collectionPath = "requests"
		}
		result, err := workspace.RunAll(collectionPath, workspace.RunOptions{
			Root:    p.Root,
			EnvName: p.EnvName,
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
			Snapshot: p.Snapshot,
		})
		if err != nil {
			base.Error = err.Error()
			return base
		}
		base.Result = result
		return base

	case "shutdown":
		base.Result = map[string]string{"status": "bye"}
		return base

	default:
		base.Error = fmt.Sprintf("unknown method: %s", req.Method)
		return base
	}
}

// --- One-shot commands (CLI backward compat) ---

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

func runInitDemo(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("init-demo", flag.ContinueOnError)
	fs.SetOutput(stderr)

	root := fs.String("root", ".", "workspace root")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := app.InitDemo(*root, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	return 0
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
