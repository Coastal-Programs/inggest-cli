---
name: test
description: Run all tests with race detector, then spawn parallel agents to fix any failures
---

## Step 1: Run all tests

```bash
go test -race -v ./... 2>&1
```

## Step 2: Analyse results

If all tests pass, report success and stop.

If there are failures, parse the output and group by package:
- `internal/inngest` — API client, GraphQL, REST, dev server
- `internal/cli/commands` — command behaviour
- `internal/common/config` — config load/save/resolve
- `pkg/output` — formatter output

## Step 3: Fix failures

For each failing package, spawn a parallel agent using the Task tool in a SINGLE response with MULTIPLE Task tool calls.

Each agent should:
1. Read the failing test file and the source file it tests
2. Determine whether the source or the test is wrong
3. Fix the issue
4. Run `go test -race ./internal/<package>/...` to verify
5. Report completion

## Step 4: Verify

After all agents complete:
```bash
go test -race ./...
```

Confirm all tests pass with zero failures.
