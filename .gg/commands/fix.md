---
name: fix
description: Run typechecking and linting, then spawn parallel agents to fix all issues
---

Run all linting and typechecking tools, collect errors, group them by domain, and use the subagent tool to spawn parallel sub-agents to fix them.

## Step 1: Run Checks

Run each tool and capture output:
- `gofmt -l .` (format errors — files that need formatting)
- `go vet ./...` (type/correctness errors)
- `golangci-lint run ./...` (lint errors — same linter as `make check`)
- `go test ./... -count=1` (test failures)

Note: `make fix` auto-fixes the mechanical subset (`gofmt -w` +
`golangci-lint run --fix`) first — run it before triaging so only the
issues that need real work remain.

## Step 2: Collect and Group Errors

Parse the output. Group errors by domain:
- **Format errors**: files listed by `gofmt -l`
- **Vet errors**: issues from `go vet`
- **Lint errors**: issues from `golangci-lint`
- **Test failures**: failing tests from `go test`

## Step 3: Spawn Parallel Agents

First run `make fix` (`gofmt -w .` + `golangci-lint run --fix`) to clear
everything auto-fixable. Then, for each domain with remaining issues, use
the subagent tool to spawn a sub-agent to fix all errors in that domain.
Include the full error output and file paths in each agent's task.

## Step 4: Verify

After all agents complete, re-run the same checks `make check` uses to
verify all issues are resolved: `golangci-lint run ./...`, `go vet ./...`,
and `go test ./... -count=1`.
