---
name: update
description: Update Go dependencies, fix deprecations and security issues
---

## Step 1: Check for Updates

List all dependencies and check for available updates:

```
go list -m -u all
```

Review the output. Modules with `[v1.X.Y]` annotations have newer versions available.

## Step 2: Update Dependencies

Update all direct and indirect dependencies to their latest minor/patch versions:

```
go get -u ./...
go mod tidy
go mod verify
```

Then check for known vulnerabilities:

```
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

If govulncheck reports any findings, update the affected module to the fixed version it recommends and re-run until clean.

## Step 3: Check for Deprecations & Warnings

Run a full build and read ALL output carefully:

```
go build ./...
go vet ./...
```

Look for:
- Deprecated function/type usage (e.g. `io/ioutil`, `strings.Title`)
- Vet warnings about incorrect format strings, unreachable code, struct tags
- Build warnings about minimum Go version compatibility

Also run staticcheck if available:

```
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...
```

## Step 4: Fix Issues

For each deprecation or warning found:
1. Research the recommended replacement (e.g. `io/ioutil.ReadAll` → `io.ReadAll`)
2. Update the code
3. Re-run `go vet ./...` and `staticcheck ./...`
4. Verify no warnings remain

For security vulnerabilities:
1. Update to the patched version `govulncheck` recommends
2. If no patch exists, evaluate the risk and document it
3. Re-run `govulncheck ./...` until clean

## Step 5: Run Quality Checks

Run the full project quality gate:

```
go fmt ./...
go vet ./...
go test -race -cover ./...
```

If golangci-lint is available:

```
golangci-lint run ./...
```

Fix all errors before completing. Coverage must remain at 100% on:
- `pkg/output`
- `internal/inngest`
- `internal/common/config`
- `internal/cli/commands`

## Step 6: Verify Clean Build

Clear module cache artifacts and verify everything resolves cleanly:

```
go clean -cache -testcache
go mod download
go build ./...
go test -count=1 ./...
```

Confirm zero warnings and zero errors. Run `make build` to verify the final binary compiles.
