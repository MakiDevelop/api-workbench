# Tutorial Draft

## Replace Postman Smoke Checks with Git-Native API Files

This draft is intended for a future blog post, launch thread, or dev.to article.

### Hook

Most API checks die in one of two places:

- inside a GUI client that nobody reviews
- inside shell scripts that nobody wants to maintain

`api-workbench` takes a different approach. Request checks live as files in Git, run from the CLI, and fit naturally into CI.

### Setup

```bash
go install github.com/MakiDevelop/api-workbench/cmd/apiw@latest
mkdir demo-checks
cd demo-checks
apiw init
```

Set the base URL:

```env
BASE_URL=https://httpbin.org
```

### First Request

```json
{
  "name": "status-check",
  "method": "GET",
  "url": "${BASE_URL}/status/200",
  "assertions": [
    {
      "type": "status",
      "equals": 200
    }
  ]
}
```

Run it:

```bash
apiw run requests/status-check.json --env local --snapshot
```

### Why This Workflow Is Better

- request definitions are code-reviewed
- API checks can run in CI without special export steps
- snapshots create a baseline for later diffs
- collections can be organized like the rest of your repository

### Next Demo Ideas

- run a whole `requests/smoke/` collection
- import a `curl` command into a request spec
- compare snapshots between staging and production
