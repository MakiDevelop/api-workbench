# Onboarding Guide

This guide is optimized for the first 10 minutes with `api-workbench`.

## Goal

Get from zero to a working repo-native API check with:

- one env file
- one request spec
- one collection run

## 1. Build the CLI

```bash
git clone https://github.com/MakiDevelop/api-workbench.git
cd api-workbench
go build -o bin/apiw ./cmd/apiw
```

## 2. Start a fresh workspace

```bash
mkdir demo-checks
cd demo-checks
/path/to/api-workbench/bin/apiw init
```

## 3. Point the workspace at an API

Edit `.apiw/env/local.env`:

```env
BASE_URL=https://httpbin.org
```

## 4. Run the generated request

```bash
/path/to/api-workbench/bin/apiw run requests/health.json --env local --snapshot
```

## 5. Create a small collection

Add `requests/headers.json`:

```json
{
  "name": "headers-check",
  "method": "GET",
  "url": "${BASE_URL}/headers",
  "assertions": [
    {
      "type": "status",
      "equals": 200
    },
    {
      "type": "body_contains",
      "contains": "headers"
    }
  ]
}
```

Run the whole collection:

```bash
/path/to/api-workbench/bin/apiw run --all requests --env local --snapshot
```

## Suggested First Real Use Cases

- health check smoke tests before deploy
- contract checks for staging APIs
- versioned regression snapshots in a backend repo
- replacing one-off Postman collections with reviewable files

## Recommended Repo Layout

```text
.apiw/
  apiw.json
  env/
    local.env
    staging.env
  snapshots/
requests/
  smoke/
  auth/
  admin/
```

## Next Step

If you want a copyable demo setup, start with [`examples/httpbin/README.md`](../examples/httpbin/README.md).
