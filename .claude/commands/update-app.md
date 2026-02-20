---
name: update-app
description: Update Go dependencies, tidy modules, and check for vulnerabilities
---

## Step 1: Check what's outdated

```bash
go list -u -m all 2>/dev/null | grep '\['
```

## Step 2: Update dependencies

```bash
go get -u ./...
go mod tidy
go mod verify
```

## Step 3: Check for vulnerabilities

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

Review any reported vulnerabilities and update affected packages.

## Step 4: Run quality checks

```bash
make check
```

Fix all errors before continuing.

## Step 5: Verify clean build

```bash
make build
./build/xero version
```

Confirm the binary builds and runs cleanly with zero warnings.
