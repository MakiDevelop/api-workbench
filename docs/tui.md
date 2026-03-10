# TUI Guide

`apiw tui` is the first interactive layer on top of the existing CLI and runner.

It is intentionally minimal:

- no external UI dependencies
- no raw-mode key handling
- no mouse support
- line-oriented commands only

That keeps the interaction model simple while reusing the existing execution engine.

## Start

Run it from an initialized workspace:

```bash
apiw tui
```

Or point it at a specific request directory:

```bash
apiw tui requests/smoke --env staging --snapshot
```

## Commands

- `[number]`: select a request by index
- `r`: run the selected request
- `a`: run all requests in the current collection
- `e <env>`: switch environment by name or 1-based index
- `s`: toggle snapshot writing on or off
- `reload`: refresh env files and request files from disk
- `q`: quit
- `help`: show the command summary

## Why This Version Exists

The current TUI is a stepping stone.

It proves that:

- the execution engine can support an interactive workflow
- request selection and env switching feel natural in-terminal
- future UI layers can be added without changing the request format

## Next TUI Improvements

- raw-mode key handling
- split panes for request list and output
- inline request preview
- snapshot diff view
- optional color and keyboard navigation
