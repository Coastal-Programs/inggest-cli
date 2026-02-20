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
- Auth layer uses `authHTTPClient` (15s timeout) — don't create new clients in auth code
- Xero API layer uses the shared transport in `internal/xero/client.go`

## URLs
- Query params: always `url.Values{}` and `.Encode()` — never string concatenation
- Path segments: `url.PathEscape(id)` for any user-supplied ID in a URL path

## Output
- Every command returns JSON by default via `output.Print(data, output.FormatJSON)`
- Errors always go to stderr via `output.PrintError(msg, err)`
- Respect the `--output` flag — never hard-code a format inside a command

## Xero API
- Xero pagination is 1-indexed — never pass `page=0` to the API
- `page=0` in CLI flags means auto-paginate: loop until a batch returns < 100 records
- 429 rate limit handling lives in `client.go` — don't add retry logic elsewhere

## Adding a New Command
1. `internal/xero/<resource>.go` — API client methods
2. `internal/cli/commands/<resource>.go` — Cobra command(s)
3. Register in `internal/cli/root.go`
4. Add to Commands section of `README.md`
5. Add `[Unreleased]` entry to `CHANGELOG.md`
