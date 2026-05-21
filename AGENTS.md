# AGENTS.md

Guidance for AI coding agents working in this repository.
See [`CLAUDE.md`](./CLAUDE.md) for the canonical, detailed agent rules
(`.claude/rules/` covers Go style, security, and release process).

## Project overview

Go CLI for the Inngest platform — monitor, debug, and manage Inngest functions
from the terminal. Built for AI agents, shell scripts, and CI/CD pipelines.

- **Module:** `github.com/Coastal-Programs/inggest-cli`
- **Binary:** `inngest`
- **Config:** `~/.config/inngest/cli.json`
- **External deps:** `github.com/spf13/cobra`, `golang.org/x/term` — prefer stdlib otherwise.

## Build & test commands

```bash
make build       # → ./build/inngest
make install     # → $GOPATH/bin
make test        # go test with race detector + coverage
make fmt         # gofmt -w (mutating fixer)
make fix         # gofmt -w + golangci-lint --fix (mutating auto-fixer)
make fmt-check   # gofmt -l (non-mutating, fails if unformatted)
make vet         # go vet ./...
make lint        # golangci-lint run
make check       # fmt-check + vet + lint — non-mutating pre-commit gate
make hooks       # install git hooks via lefthook (opt-in)
```

Run `make check` and `make test` before committing.

## Code style

- `gofmt` is non-negotiable; `golangci-lint` config lives in `.golangci.yml`.
- Wrap errors with context (`fmt.Errorf("...: %w", err)`).
- Prefer the standard library; new external dependencies need justification.
- One file per command group under `internal/cli/commands/`.

## Project structure

```
cmd/inngest/main.go                  # entry point, ldflags: version
internal/cli/root.go                 # root Cobra command, --env/--output/--dev flags
internal/cli/commands/               # one file per command group
internal/inngest/                    # Inngest API client (GraphQL + REST + dev server)
internal/common/config/config.go     # config load/save, env var fallbacks
pkg/output/output.go                 # JSON / text / table formatter
scripts/release.sh                   # cross-platform release builder
```

## Testing instructions

- CI workflows live in `.github/workflows/` (`test.yml`, `release.yml`).
- `make test` runs the full suite with `-race` and coverage.
- Add or update tests for any code you change.
