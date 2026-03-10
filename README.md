# api-workbench

CLI-first API workbench for repo-native API testing and automation.

## Why

`api-workbench` is built around a simple idea:

- Requests should live in a repository, not inside a desktop app.
- Terminal usage should be the default, not an afterthought.
- The same request definition should run locally and in CI.
- Snapshots should make API behavior changes easy to inspect.

This first milestone focuses on a narrow MVP:

- `apiw init` creates a repo-friendly project layout.
- `apiw run` executes a request spec from disk.
- Environment values come from `.env`-style files plus process env vars.
- Assertions validate the response.
- Snapshots can be written to disk for later diffing.

## Current MVP Scope

The current request format is JSON-only by design. YAML and collection-level commands can come later once the execution core is stable.

Implemented now:

- Project bootstrap
- Request execution
- Collection execution via `apiw run --all`
- Header / query / body templating via `${VAR}`
- Basic assertions
- Snapshot writing

Planned next:

- OpenAPI / `curl` import
- Snapshot diff
- Machine-readable CI output
- Optional TUI / desktop shell

## Quick Start

```bash
go run ./cmd/apiw init
go run ./cmd/apiw run requests/health.json --env local --snapshot
go run ./cmd/apiw run --all --env local
```

Generated structure:

```text
.apiw/
  apiw.json
  env/
    local.env
  snapshots/
requests/
  health.json
```

## Request Spec

Example `requests/health.json`:

```json
{
  "name": "health-check",
  "method": "GET",
  "url": "${BASE_URL}/health",
  "headers": {
    "Accept": "application/json"
  },
  "query": {
    "source": "apiw"
  },
  "assertions": [
    {
      "type": "status",
      "equals": 200
    }
  ]
}
```

Supported assertion types:

- `status`
- `body_contains`
- `header_equals`

Optional body format:

```json
{
  "body": {
    "type": "json",
    "content": {
      "message": "hello"
    }
  }
}
```

For plain text:

```json
{
  "body": {
    "type": "text",
    "content": "hello"
  }
}
```

## Commands

```bash
apiw init
apiw run requests/health.json --env local
apiw run --all --env staging
apiw run requests/create-user.json --env staging --snapshot
```

See [docs/mvp.md](/Users/maki/GitHub/api-workbench/docs/mvp.md), [docs/cli.md](/Users/maki/GitHub/api-workbench/docs/cli.md), and [docs/roadmap.md](/Users/maki/GitHub/api-workbench/docs/roadmap.md) for the current product and strategy direction.
