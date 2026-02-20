---
name: fix
description: Run typechecking and linting, then spawn parallel agents to fix all issues
---

# Project Code Quality Check

Run all linting and typechecking tools, collect errors, group by domain, and spawn parallel agents to fix them.

## Step 1: Run Linting and Typechecking

```bash
npm run typecheck 2>&1
npm run lint 2>&1
npm run format:check 2>&1
```

## Step 2: Collect and Parse Errors

Parse the output. Group errors by domain:
- **Type errors**: TypeScript compiler errors from `tsc --noEmit`
- **Lint errors**: ESLint issues from `eslint src/ tests/`
- **Format errors**: Prettier issues from `prettier --check`

Create a list of all files with issues and the specific problems in each file.

## Step 3: Spawn Parallel Agents

For each domain that has issues, spawn an agent in parallel using the Task tool in a SINGLE response with MULTIPLE Task tool calls:

- Spawn a "type-fixer" agent for type errors with the list of files and errors
- Spawn a "lint-fixer" agent for lint errors with the list of files and errors
- Spawn a "format-fixer" agent that runs `npm run format` to auto-fix formatting

Each agent should:
1. Receive the list of files and specific errors in their domain
2. Fix all errors
3. Run the relevant check command to verify fixes
4. Report completion

## Step 4: Verify All Fixes

After all agents complete, run the full check:
```bash
npm run check
```
Ensure all issues are resolved. If any remain, fix them directly.
