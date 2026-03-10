# Desktop GUI Guide

`api-workbench` now includes a Tauri-based desktop GUI for macOS and Windows.

The desktop app is a shell around the existing Go execution engine:

- the GUI is built with Tauri + Vite
- the execution backend is a packaged Go sidecar
- request definitions and env files stay exactly the same

## Current Scope

The first desktop release supports:

- selecting a workspace folder
- listing env files
- listing request specs
- running a selected request
- running a whole collection
- toggling snapshot generation

This keeps the GUI aligned with the existing CLI and TUI instead of inventing a separate workflow.

## Development

Install dependencies:

```bash
npm install
```

Start the desktop app in dev mode:

```bash
npm run tauri:dev
```

## Packaging

Build a macOS app bundle locally:

```bash
npm run tauri:build:mac
```

Output:

```text
src-tauri/target/release/bundle/macos/API Workbench.app
```

For Windows, this repo uses GitHub Actions on native Windows runners. That avoids pretending a macOS machine can reliably produce a Windows installer locally.

Workflow:

- `.github/workflows/desktop-bundles.yml`

Artifacts:

- macOS bundle
- Windows bundle

## Architecture

```text
Vite frontend
  -> Tauri invoke commands
  -> Rust bridge
  -> packaged Go sidecar
  -> existing request/runner logic
```

## Next Desktop Steps

- request preview pane
- collection summary charts
- snapshot diff screen
- richer keyboard navigation
- optional Tauri updater flow
