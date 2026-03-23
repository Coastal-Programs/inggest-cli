---
paths:
  - "**/*.go"
---

# Go Conventions

## Errors
- Wrap with `fmt.Errorf("context: %w", err)` — never discard or swallow errors
- User-facing errors go to stderr via `output.PrintError()`, not `fmt.Println`

## HTTP
- Always set an explicit timeout — never use `http.DefaultClient` bare
- Inngest API layer uses the shared transport in `internal/inngest/client.go`

## URLs
- Query params: always `url.Values{}` and `.Encode()` — never string concatenation
- Path segments: `url.PathEscape(id)` for any user-supplied ID in a URL path

## Output
- Every command returns JSON by default via `output.Print(data, output.FormatJSON)`
- Errors always go to stderr via `output.PrintError(msg, err)`
- Respect the `--output` flag — never hard-code a format inside a command

## Inngest API
- GraphQL API at `api.inngest.com/gql` — used for functions, runs, environments, metrics
- REST API at `api.inngest.com/v1/` — used for sending events
- Dev server at `localhost:8288` — no auth required
- Three auth modes: signing key (GraphQL/REST), event key (events), no auth (dev server)

## Adding a New Command
1. `internal/inngest/<resource>.go` — API client methods (or add to `client.go`)
2. `internal/cli/commands/<resource>.go` — Cobra command(s)
3. Register in `internal/cli/root.go`
4. Add to Commands section of `README.md`
5. Add `[Unreleased]` entry to `CHANGELOG.md`
