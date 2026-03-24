---
name: fix
description: Run typechecking and linting, then spawn parallel agents to fix all issues
---

Run all linting and typechecking tools, collect errors, group them by domain, and use the subagent tool to spawn parallel sub-agents to fix them.

## Step 1: Run Checks

Run each tool and capture output:
- `gofmt -l .` (format errors — files that need formatting)
- `go vet ./...` (type/correctness errors)
- `staticcheck ./...` (lint errors — unused code, simplifications, bugs)
- `go test ./... -count=1` (test failures)

## Step 2: Collect and Group Errors

Parse the output. Group errors by domain:
- **Format errors**: files listed by `gofmt -l`
- **Vet errors**: issues from `go vet`
- **Lint errors**: issues from `staticcheck`
- **Test failures**: failing tests from `go test`

## Step 3: Spawn Parallel Agents

For each domain with issues, use the subagent tool to spawn a sub-agent to fix all errors in that domain. Include the full error output and file paths in each agent's task.

Auto-fix format errors directly with `gofmt -w .` instead of spawning an agent.

## Step 4: Verify

After all agents complete, re-run all checks to verify all issues are resolved.
