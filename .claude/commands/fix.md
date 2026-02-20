---
name: fix
description: Run vet and lint, then spawn parallel agents to fix all issues
---

## Step 1: Run checks

```bash
go vet ./... 2>&1
golangci-lint run ./... 2>&1
```

## Step 2: Collect and group errors

Parse the output and group by type:
- **Vet errors** — incorrect API usage, unreachable code, bad format strings
- **Lint errors** — style violations, unused variables, error handling issues
- **Build errors** — compilation failures

## Step 3: Spawn parallel agents

For each category that has issues, spawn a parallel agent using the Task tool in a SINGLE response with MULTIPLE Task tool calls:

- Spawn a **vet-fixer** agent with the list of vet errors and affected files
- Spawn a **lint-fixer** agent with the list of lint errors and affected files

Each agent should:
1. Read the affected files
2. Fix all errors in that category
3. Re-run the relevant check to verify
4. Report completion

## Step 4: Verify

After all agents complete:
```bash
make check
```

Confirm zero errors remain.
