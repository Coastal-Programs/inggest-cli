---
name: test
description: Run all tests, then spawn parallel agents to fix any failures
---

## Step 1: Run All Tests

```bash
npm run test 2>&1
```

## Step 2: Analyze Results

If all tests pass, report success and stop.

If there are failures, parse the output and group by test file:
- `tests/unit/app.test.tsx` — React component tests (jsdom)
- `tests/unit/ipc.test.ts` — IPC handler tests (node)
- `tests/unit/preload.test.ts` — Preload script tests (node)
- `tests/unit/main-process.test.ts` — Main process tests (node)

## Step 3: Fix Failures

For each failing test file, spawn a parallel agent using the Task tool with the list of failures and the relevant source + test files.

Each agent should:
1. Read the failing test file and corresponding source file
2. Fix the issue (in source or test, whichever is wrong)
3. Run `npx vitest run <test-file>` to verify the fix

## Step 4: Verify

After all agents complete, run the full suite:
```bash
npm run test
```

Confirm all 37+ tests pass with zero failures.

## Options

- Watch mode: `npm run test:watch`
- Single file: `npx vitest run tests/unit/app.test.tsx`
- Filter by name: `npx vitest run -t "renders the app title"`
