# Growth Strategy

This document adapts the GitHub growth ideas from `/Users/maki/Documents/side_project_blueprint_v3.md` to `api-workbench`.

## Growth Positioning

`api-workbench` should aim to be:

- the repo-native alternative to GUI API clients
- the easiest way to turn API checks into reviewable Git artifacts
- the bridge between local debugging and CI smoke tests

## Growth Drivers

To attract stars and adoption, the repo needs four things:

1. clear positioning
2. low-friction onboarding
3. strong examples and docs
4. visible developer experience quality

## Repo Checklist

Current:

- README with positioning
- architecture diagram
- onboarding guide
- example collection
- CI workflow

Next:

- animated terminal demo or GIF
- comparison page vs Postman / Bruno / Insomnia
- `curl` import demo
- snapshot diff demo
- machine-readable output examples for CI

## Audience

Primary users:

- backend engineers
- platform engineers
- QA engineers with Git-based workflows

Secondary users:

- teams leaving Postman for repo-native checks
- OSS maintainers who want smoke tests in their repositories

## Launch Narrative

The message should stay simple:

> Stop hiding API checks inside GUI state. Put them in Git and run them anywhere.

## Content Plan

High-signal content for launch:

- a tutorial post showing how to replace a Postman smoke collection
- a short terminal demo clip
- a real-world CI example
- a minimal example repo that anyone can copy

## GitHub Hygiene

- keep the README short enough to scan quickly
- keep examples runnable without a database or external services
- keep CI green
- publish small, frequent releases instead of one giant roadmap promise

## Star Strategy

Short-term:

- make onboarding crisp enough that people star after one successful run

Mid-term:

- ship `curl` import and snapshot diff
- publish comparison content and a launch post

Long-term:

- become the default "API checks in Git" recommendation for CLI-first teams
