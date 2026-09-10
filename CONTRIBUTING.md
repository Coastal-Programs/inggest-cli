# Contributing to inngest-cli

Thanks for your interest in contributing. This document covers how to set up
the project, the standards a change needs to meet, and how to get it merged.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Code Style](#code-style)
- [Testing Requirements](#testing-requirements)
- [Commit Messages](#commit-messages)
- [Pull Request Process](#pull-request-process)
- [Project Structure](#project-structure)
- [Reporting Issues](#reporting-issues)

## Code of Conduct

Be respectful, constructive and collaborative. Contributions are welcome from
everyone.

## Getting Started

1. **Fork** the repository on GitHub.
2. **Clone** your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/inggest-cli.git
   cd inggest-cli
   ```
3. **Add the upstream remote**:
   ```bash
   git remote add upstream https://github.com/Coastal-Programs/inggest-cli.git
   ```
4. **Create a branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Setup

### Prerequisites

- **Go** — the version in `go.mod` (currently 1.26.8) or later
- **Make**
- **Git**
- **golangci-lint** — required for `make lint` and `make check`
- **lefthook** — optional, for git hooks

### Build and test

```bash
go mod download   # fetch dependencies
make build        # build to ./build/inngest
make test         # full suite with race detector and coverage
```

`make install` puts the binary in `$GOPATH/bin` instead.

### Git hooks (optional)

```bash
make hooks        # installs a pre-commit (gofmt + vet) and pre-push (test) gate
```

Hooks are opt-in and managed by [lefthook](https://lefthook.dev).

## Code Style

`gofmt` is non-negotiable. The linter config lives in `.golangci.yml`.

```bash
make fmt          # gofmt -w (mutating)
make fmt-check    # gofmt -l (non-mutating, fails if unformatted)
make fix          # gofmt -w + golangci-lint --fix (mutating auto-fixer)
make vet          # go vet ./...
make lint         # golangci-lint run
make check        # fmt-check + vet + lint — run this before every commit
```

Conventions:

- **Wrap errors with context**: `fmt.Errorf("listing runs: %w", err)`.
- **Prefer the standard library.** A new external dependency needs
  justification — the CLI ships with 2 direct dependencies and we intend to
  keep that number near zero.
- **One file per command group** under `internal/cli/commands/`.
- **Never log or print a credential.** Use `config.Redact()` for anything that
  echoes a key back to the user.

## Testing Requirements

```bash
make test                      # everything, with -race and coverage
go test ./internal/inngest/    # a single package
go test -run TestListRuns ./internal/inngest/
```

Requirements for any change:

1. **Add or update tests** for the code you touch.
2. **Tests must be hermetic.** Never call the live Inngest API. Use
   `httptest` servers; the existing helpers in each package's
   `testhelpers_test.go` cover the common cases.
3. **Assert real behaviour** — status codes, error messages, decoded fields,
   headers sent. A test that executes a line without asserting anything about
   it does not count as coverage.
4. **Do not weaken a check to get green.** No `t.Skip`, no blanket `//nolint`,
   no deleted assertions. If a branch is genuinely unreachable, say so in a
   comment rather than contorting the code to reach it.

Coverage is high by design (`internal/inngest`, `internal/common/config`,
`internal/cli` and `pkg/output` are at 100%). Please do not regress it.

## Commit Messages

- **Subject**: one line, starting with `Add`, `Update`, `Fix`, `Remove` or
  `Refactor`. Release commits use `chore: release vX.Y.Z`.
- **Body**: short bullets grouped under `Added:`, `Updated:`, `Fixed:` or
  `Docs:`. Omit empty sections.
- Bullets should name concrete files, functions or behaviours — not vague
  summaries.

Example:

```
Fix run cancellation against the v2 API

Fixed:
- CancelRun posted to the v1 path, returning 404 for every cloud run
- The error omitted the run ID, making failures hard to trace
```

## Pull Request Process

1. Run `make check` and `make test`; both must pass.
2. Update `README.md` or `ARCHITECTURE.md` if you changed behaviour or added a
   command.
3. Add a `CHANGELOG.md` entry under the appropriate heading.
4. Open the PR against `main` with a clear description of the problem and how
   you solved it.
5. CI (`.github/workflows/test.yml`) must be green before merge.

Maintainers handle version bumps, tagging and npm publishing — please do not
bump versions in a PR.

## Project Structure

```
cmd/inngest/main.go                # entry point; version injected via ldflags
internal/cli/root.go               # root command, global flags
internal/cli/commands/             # one file per command group
internal/inngest/                  # API client (REST v2, v1 events, dev GraphQL)
internal/common/config/            # config load/save and env var fallbacks
pkg/output/                        # JSON / text / table formatter
scripts/release.sh                 # cross-platform release builder
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for how these fit together.

## Reporting Issues

- **Bugs and features**: use the
  [issue templates](https://github.com/Coastal-Programs/inggest-cli/issues/new/choose).
  Include the command you ran, what you expected, the actual output and your
  `inngest version`.
- **Security vulnerabilities**: do **not** open a public issue. Follow
  [SECURITY.md](SECURITY.md).
