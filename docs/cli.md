# CLI Design

## Command Philosophy

Commands should be short, deterministic, and automation-friendly.

Output rules:

- Human-readable by default
- Stable exit code semantics
- Compact summaries first, details after

## Initial Commands

### `apiw init`

Creates the base project structure:

- `.apiw/apiw.json`
- `.apiw/env/local.env`
- `.apiw/snapshots/`
- `requests/health.json`

Behavior:

- Safe to re-run
- Existing files are preserved
- Reports created vs skipped paths

### `apiw run <request-file>`

Runs a single request spec.

Flags:

- `--env <name>`: loads `.apiw/env/<name>.env`
- `--snapshot`: writes `.apiw/snapshots/<request>--<env>.json`
- `--timeout <duration>`: request timeout, default `15s`
- `--all`: runs every JSON request spec under a collection directory

Examples:

```bash
apiw run requests/health.json --env local
apiw run --all --env staging
apiw run --all requests/smoke --env local --snapshot
```

Exit codes:

- `0`: request succeeded and assertions passed
- `1`: CLI or validation error
- `2`: transport error
- `3`: assertion failure

## Request Resolution Rules

- Request file paths are resolved from the current working directory.
- `apiw run --all` defaults to the `requests/` directory.
- `${VAR}` placeholders are expanded from:
  - selected env file
  - process environment variables
- Process environment variables override file values.

## Collection Output

`apiw run --all` prints each file as it runs, then prints a final summary:

```text
summary        total=3 passed=2 failed=1 transport=0 invalid=0
```

## Next Commands

The next likely additions after MVP:

- `apiw import curl`
- `apiw import openapi`
- `apiw snapshot diff`
