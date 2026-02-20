# Zeus CLI — Project Memory

## Overview

Zeus CLI (`xero`) is a Go-based command-line tool for the Xero Accounting API.
Built for AI agents and financial data pipelines serving Australian retirement agencies (multi-org).

- **Module:** `github.com/jakeschepis/zeus-cli`
- **Binary name:** `xero`
- **Config file:** `~/.config/xero/config.json` (0600 permissions)
- **Xero API base:** `https://api.xero.com/api.xro/2.0`

## Essential Commands

```bash
make build      # Build to ./build/xero
make install    # Install to $GOPATH/bin
make test       # Run all tests
make vet        # Run go vet
make fmt        # Run go fmt
make check      # fmt + vet + test (run before committing)
make clean      # Remove build/dist artifacts
```

Test the CLI locally after building:

```bash
./build/xero auth status
./build/xero orgs list
./build/xero invoices list --status AUTHORISED
```

## Project Structure

```
cmd/xero/main.go                  # Entry point — passes version to cli.Execute()
internal/cli/root.go              # Root Cobra command, global flags (--org, --output)
internal/cli/commands/            # One file per command group
internal/xero/                    # Xero API client and resource methods
internal/auth/oauth.go            # OAuth 2.0 PKCE flow + callback page
internal/auth/assets/logo.png     # Zeus logo (embedded into binary)
internal/common/config/config.go  # Config load/save, tenant resolution
pkg/output/output.go              # JSON/text/table output formatter
scripts/release.sh                # Cross-platform release build script
```

## Code Conventions

- **Error handling:** wrap with `fmt.Errorf("context: %w", err)` — never discard errors
- **HTTP clients:** always set a timeout; never use `http.DefaultClient` without one
- **URL params:** always use `url.Values{}` — never string concatenation
- **Path segments:** use `url.PathEscape(id)` for user-supplied IDs in URL paths
- **Output:** all commands return JSON by default via `output.Print()`; errors go to stderr via `output.PrintError()`
- **Pagination:** Xero API is 1-indexed; page=0 means auto-paginate (fetch all)
- **Rate limits:** `client.go` handles 429 with `Retry-After` — don't add extra retry logic elsewhere

## Git Workflow

### Commit Messages (Conventional Commits)

```
feat: add aged-payables report command
fix: use url.Values for query params in accounts.go
docs: update README installation section
chore: bump cobra to v1.9.0
refactor: extract pagination helper into client.go
```

Format: `type: short description` (imperative, lowercase, no period)

Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `ci`

### Branching

- `main` — always stable and releasable
- Feature branches: `feat/add-budgets-command`
- Fix branches: `fix/pagination-bank-transactions`

### Before Every Commit

```bash
make check   # runs fmt + vet + test
```

## Release Process

Zeus CLI uses **Semantic Versioning** (`vMAJOR.MINOR.PATCH`):

| Change type | Example | Version bump |
|------------|---------|-------------|
| Bug fix | Fix wrong page parameter | PATCH: `v0.1.1` |
| New command | Add `xero budgets` | MINOR: `v0.2.0` |
| Breaking change | Rename flag, change output shape | MAJOR: `v1.0.0` |

### Cutting a Release

1. Update `CHANGELOG.md` — move items from `[Unreleased]` to the new version section
2. Commit: `git commit -m "chore: release v0.2.0"`
3. Create an **annotated** tag:
   ```bash
   git tag -a v0.2.0 -m "Release v0.2.0"
   git push origin v0.2.0
   ```
4. GitHub Actions (`.github/workflows/release.yml`) will automatically:
   - Build cross-platform binaries (darwin, linux, windows — amd64 + arm64)
   - Create a GitHub Release with binaries and `checksums.txt`
   - Generate release notes from merged PRs

> Never push a tag without updating CHANGELOG.md first.

## Dependencies

Only one external dependency — keep it that way unless there's a strong reason:

```
github.com/spf13/cobra v1.8.1
```

Standard library is preferred for everything else (HTTP, JSON, URL handling, embed).

## Security Rules

- Never log or print raw tokens — always use `config.Redacted()` or the `redact()` helper
- Config is written with `0600` permissions — do not change this
- `XERO_CONFIG` env var is validated (absolute path, `.json` suffix, no `..`) — do not relax validation
- All HTTP clients must have explicit timeouts set

## Adding a New Command

1. Create `internal/xero/<resource>.go` with the API client methods
2. Create `internal/cli/commands/<resource>.go` with the Cobra command(s)
3. Register the command in `internal/cli/root.go`
4. Add entries to the Commands section of `README.md`
5. Add a `[Unreleased]` CHANGELOG entry
