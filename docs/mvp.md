# MVP Plan

## Positioning

`api-workbench` is a repo-native API execution tool for engineers who want versioned request definitions, CLI ergonomics, and CI-friendly verification.

## Target User

- Backend engineers validating APIs during development
- Small teams replacing Postman collections with repo-tracked specs
- QA or platform engineers who need repeatable smoke tests in CI

## First Milestone

The first milestone proves the execution core.

Deliverables:

- Bootstrap command for local project setup
- Single request execution from a spec file
- Environment substitution using `.env` files
- Assertion engine with a small stable surface
- Snapshot capture for regression workflows

Non-goals for milestone one:

- GUI
- Team collaboration features
- OAuth flows
- Websocket / gRPC support
- OpenAPI sync engine

## Core Opinion

The CLI is the product surface. The file format is the collaboration surface.

That means the first risk to reduce is execution correctness, not interface polish.

## Proposed Milestone Sequence

### M1: Execution Core

- `init`
- `run`
- env loading
- assertions
- snapshot write

### M2: Repo Workflow

- collection-level run
- exit summary for CI
- snapshot diff
- machine-readable output

### M3: Imports

- `curl` import
- OpenAPI import
- HAR import

### M4: UX Shell

- TUI or lightweight desktop wrapper
- richer history inspection
- replay / compare flows
