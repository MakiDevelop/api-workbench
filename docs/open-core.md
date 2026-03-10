# Open-Core Strategy

This document adapts the monetization ideas from `/Users/maki/Documents/side_project_blueprint_v3.md` to `api-workbench`.

## Principle

Keep the core developer workflow open.

That means the open-source project should always cover:

- local CLI execution
- repo-native request definitions
- environment files
- assertions
- collection runs
- snapshots
- importers that reduce migration cost

## Future Paid Surface

If `api-workbench` grows into a product, the paid surface should start where team-scale operational problems begin.

Good candidates:

- hosted execution history
- team workspaces
- environment secret management
- flaky endpoint analytics
- API drift dashboards
- access policies and audit logs

## What Should Stay Open

These features are too close to the core value proposition to lock away:

- running requests from files
- storing collections in Git
- checking assertions in CI
- capturing and diffing snapshots

If those become paid-only, the project loses trust and distribution power.

## Packaging Direction

Possible long-term packaging:

- Open source CLI and file format
- Hosted dashboard for teams
- Optional cloud runner for scheduled checks
- Paid analytics and governance add-ons

## Monetization Guardrails

- never require the hosted product to use the OSS CLI
- keep migration paths one-way and transparent
- avoid proprietary file formats
- avoid gating the best solo-developer workflow behind a paywall
