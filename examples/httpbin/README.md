# httpbin Example

This example gives new users a copyable first run.

## Structure

```text
examples/httpbin/
  .apiw/env/local.env.example
  requests/status-200.json
  requests/headers.json
```

## Usage

```bash
mkdir demo-httpbin
cd demo-httpbin
/path/to/api-workbench/bin/apiw init
cp /path/to/api-workbench/examples/httpbin/.apiw/env/local.env.example .apiw/env/local.env
cp /path/to/api-workbench/examples/httpbin/requests/*.json requests/
/path/to/api-workbench/bin/apiw run --all requests --env local --snapshot
```

## What It Demonstrates

- simple status assertions
- body content assertions
- collection runs
- snapshot generation
