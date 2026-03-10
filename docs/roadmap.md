# Roadmap

This roadmap adapts the direction in `/Users/maki/Documents/advanced_side_project_strategy.md` to the current `api-workbench` codebase.

## Strategic Position

`api-workbench` aims for the "quick traction" lane:

- Replace paid API clients for engineers who prefer repos over GUI state
- Win on CLI ergonomics, CI compatibility, and versioned request specs
- Add AI-assisted imports and workflow suggestions later, not first

## Product Architecture

Current architecture follows the strategy document, but adapted to a Go-first CLI:

```text
CLI
  -> request loader
  -> execution engine
  -> assertion runner
  -> snapshot writer
  -> optional future TUI / GUI
```

Current code mapping:

- `cmd/apiw`: CLI entrypoint
- `internal/request`: request spec loading and validation
- `internal/runner`: HTTP execution, assertions, snapshots
- `internal/app`: command orchestration

## 8-Week Plan

### Week 1-2: Execution Core

Status: mostly complete

- project bootstrap
- single-request execution
- env loading
- assertions
- snapshot capture

### Week 3-4: Repo Workflow

Status: in progress

- collection execution
- CI-friendly summaries
- snapshot diff
- machine-readable output

### Week 5-6: Imports

- `curl` import
- OpenAPI import
- HAR import

### Week 7-8: Power Features

- GraphQL request support
- response diff UX
- lightweight TUI or desktop shell
- AI-generated request/collection bootstrap

## Product Rules

- Keep the CLI as the primary interface
- Prefer plain-text, Git-friendly artifacts
- Delay databases and servers until the repo workflow is strong
- Add AI only where it reduces setup cost or review effort
