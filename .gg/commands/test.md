---
name: test
description: Run tests, then spawn parallel agents to fix failures
---

Run all tests for this project, collect failures, and use the subagent tool to spawn parallel sub-agents to fix them.

## Step 1: Run Tests

Run unit tests with coverage:
```
go test ./... -count=1 -cover
```

For integration tests (builds and executes the binary):
```
go test ./internal/cli/ -count=1 -tags=integration
```

For a specific package:
```
go test ./internal/inngest/ -count=1 -v
```

For a specific test:
```
go test ./internal/inngest/ -count=1 -v -run TestHashSigningKey
```

For race detection:
```
go test ./... -count=1 -race
```

## Step 2: If Failures

For each failing test, use the subagent tool to spawn a sub-agent to fix the underlying issue (not the test). Include the test name, file path, and full error output.

## Step 3: Re-run

Re-run `go test ./... -count=1 -cover` to verify all fixes and confirm coverage hasn't regressed.
