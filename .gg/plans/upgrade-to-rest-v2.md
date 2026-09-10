# Upgrade plan: move Cloud path to REST API v2, add trace/invoke/apps, harden

## Headline finding (verified live, 2026-09-09)

`inngest runs list` is **broken against Inngest Cloud today**:

```
graphql returned status 422: Cannot query field "recent" on type "Event"
```

Everything that goes through `client.ListRuns` — `runs list|get|watch`, `metrics`,
`backlog`, `--dev runs list` — dies with schema-validation errors. Root cause: the Cloud
client depends on the **undocumented dashboard GraphQL** (`api.inngest.com/gql`), which
drifted. Second verified bug: a **bad signing key returns `[]` with exit 0** from
`functions list` (GraphQL returns 200 + empty workspace), so CI/agents can't tell
"no functions" from "not authenticated".

Inngest now publishes a **documented REST API v2** (`https://api.inngest.com/v2`,
OpenAPI at `api-docs.inngest.com/api-specs/v2.json`, docs marked "Stable
(In-development)"). The official `inngest api` CLI command is built on it. Bad auth
returns a clean `401 {"errors":[{"code":"authorization_header_missing",...}]}`.

## What v2 gives us (from the OpenAPI spec)

| Today (GraphQL hack)                                | v2 endpoint                                                | Win |
|-----------------------------------------------------|-------------------------------------------------------------|-----|
| `runs list` = flatten `events.recent(50).functionRuns` | `GET /runs?status[]&appId[]&functionId[]&from&until&timeField&order&cursor&limit≤100` | real filters + real pagination |
| `runs get` = scan up to 500 recent runs (10 pages)  | `GET /runs/{id}?includeOutput=true`                         | 1 request |
| `runs cancel` needs envID + workspace GraphQL       | `POST /runs/{id}/cancel`                                    | no env lookup |
| `runs replay` = GraphQL `rerun`                     | `POST /runs/{id}/rerun` (`fromStep` optional)               | documented |
| `functions get` = list-all then filter              | `GET /apps/{app}/functions/{fn}`                            | 1 request |
| `env list` "requires account-level auth"            | `GET /envs`                                                 | works with API key |
| `events runs` (v1 REST)                             | `GET /events/{id}/runs?includeOutput`                       | same family |
| — (missing)                                         | `GET /runs/{id}/trace`                                      | **step-by-step trace tree** |
| — (missing)                                         | `POST /apps/{app}/functions/{fn}/invoke` (`data`, `idempotencyKey`) | **invoke** |
| — (missing)                                         | `GET /apps`, `GET /apps/{id}`, `POST /apps/{id}/syncs`      | apps/sync |
| `events send` needs an Event Key                    | `POST /events` (name/data/user/id/ts) with signing/API key  | no event key needed |
| — (missing)                                         | `POST /insights/query` (SQL), `GET /insights/tables`        | later |

Run shape: `{id, status(QUEUED|RUNNING|COMPLETED|FAILED|CANCELLED), app{id,name,slug},
function{id,name,slug}, trigger{eventName,eventIds,cronSchedule,isBatch}, queuedAt,
startedAt, endedAt, durationMs, output}` + `page{cursor,hasMore,limit}`.

## Phases (each = one PR, `make check && make test` green)

### Phase 1 — fix what's broken: Cloud runs/functions/envs on v2
- `internal/inngest/restv2.go`: tiny `v2Get/v2Post` on the existing `Client.do` (`/v2/...`,
  `Accept: application/json`, `X-Inngest-Env`), decode `{data,page,metadata}` and
  `{errors:[{code,message}]}` into the existing `APIError` (keeps `IsAuthError` etc).
- Re-point `ListRuns`, `GetRun`, `CancelRun`, `RerunRun`, `ListFunctions`, `GetFunction`,
  `ListEnvironments` at v2. Delete the `events.recent` flattening and the 500-run scan.
  Keep `types.go` field names where possible so `-o table` columns and tests don't churn.
- `runs list`: wire `--app`, `--from/--until`, `--order`, `--time-field`; `--limit` max 100;
  emit `next_cursor` for `--after`.
- `runs watch`: fix the reprint bug (cursor never set, `After` ignored, `From` frozen).
  Use `from=<last queuedAt>&order=ASC` + a seen-ID set per tick.
- `metrics`/`backlog`: replace `paginateRuns` (HasNextPage was always false) with real
  cursor paging + server-side `status` filter; cap total scanned (`--sample`).
- Dev server stays on its OSS GraphQL (`dev runs` works today; local dev server v1.17.0
  has v2 `/health` + `/envs` but returns 404 for `/runs` and `/apps`). `--dev runs list`
  routes to the `dev runs` query so it stops 422-ing.
- Tests: `httptest` fixtures from the OpenAPI shapes; keep the existing mock-server pattern.

### Phase 2 — new commands (all v2, all JSON-first)
- `runs trace <id>` — render span tree as text (`step name / op / status / duration`) and JSON.
- `functions invoke <app> <fn> --data '{...}' [--data-file -] [--idempotency-key]` →
  returns `runId`; `--wait` polls `GET /runs/{id}` until terminal (reuse `runs get` poll).
- `apps list [--archived]`, `apps get <id>`, `apps sync <id> --url <serve-url>`.
- `events send`: prefer `POST /v2/events` when no event key is configured; keep the Event
  API path (batches, full fields) when one is.
- `api <METHOD> <path> [--body|--body-file -] [--raw]` — passthrough escape hatch like
  `gh api` / official `inngest api`, so agents can hit any endpoint we haven't wrapped.

### Phase 3 — auth + hardening (patterns lifted from the official CLI)
- Accept **API keys** (`INNGEST_API_KEY`, `auth login --api-key`, config `api_key`). Docs
  recommend API keys for CLI/CI/AI use; the current `validateSigningKey` rejects them.
  Precedence: `--api-key` flag > `INNGEST_API_KEY` > `INNGEST_SIGNING_KEY` > config.
- Refuse to send a bearer token over plaintext `http://` to a non-localhost host
  (`--api-url http://evil` currently ships the key) — official CLI's `guardPlaintextAuth`.
- `io.LimitReader` (25 MB) on every response body; today `io.ReadAll` is unbounded.
- `url.PathEscape` in `GetEventRuns` (violates `.claude/rules/go.md`).
- `cmd.Context()` + `signal.NotifyContext` in root, retry backoff honours ctx
  (Ctrl+C on a hung request works everywhere, not only in `watch`).

### Phase 4 — agent/CI ergonomics (small, independent)
- Distinct exit codes like `gh`: 1 error, 2 cancelled, 4 auth. Bad auth must never be exit 0.
- Global `--timeout` (default 30s) instead of the hard-coded client timeout.
- `-o text` on maps prints sorted keys (currently random order → flaky diffs).
- `health`: call `IsDevServerRunning` once, not twice.
- README/AGENTS.md: config path on macOS is `~/Library/Application Support/inngest/cli.json`
  (docs say `~/.config/...`; `config path` proves otherwise).
- `dev runs` JSON shows `functionID: ""` / `function.id: ""` — check the dev GraphQL field.

## Speed
Startup is 11 ms, 6.7 MB binary — already fast; nothing to shave there. Real speedups
come from request shape: `runs get` 10 pages → 1 request, `functions get` list-all → 1,
`metrics` N flattened pages → 1–3 filtered pages.

## Tradeoffs / unverified
- v2 is labelled "Stable (In-development)": shapes may still move, but it is documented,
  spec'd, and what the vendor's own CLI uses — strictly better than the dashboard GraphQL
  that already broke. Keep the v2 decoder tolerant (unknown fields ignored, pointers for
  optional times).
- I could not run v2 against a real signing key (the stored key looks like a placeholder);
  Phase 1 needs one real call before merge. Bad-key 401 path is verified.
- Insights (SQL), bulk cancellations (v1 `POST /cancellations`), webhooks, sandboxes: real
  value but out of scope until Phases 1–3 land.

## Out of scope
Colour output, TUI, update-checker, extra dependencies (stays cobra + x/term).

## Steps
1. Add `internal/inngest/restv2.go`: `v2Get`/`v2Post` on `Client.do`, `{data,page}` + `{errors}` decoding into `APIError`, `io.LimitReader`, `X-Inngest-Env`; unit tests with `httptest`.
2. Re-point `ListRuns`/`GetRun`/`CancelRun`/`RerunRun` at `/v2/runs...`; delete the `events.recent` flattening and 500-run scan; update `runs.go` flags (`--app`, `--from/--until`, `--order`, `--time-field`, `--after`, limit ≤100) and tests.
3. Re-point `ListFunctions`/`GetFunction`/`ListEnvironments` at `/v2/apps`, `/v2/apps/{app}/functions[/{fn}]`, `/v2/envs`; remove the `envs` account-auth caveat; update tests.
4. Fix `runs watch` dedup (`from=<last queuedAt>&order=ASC` + seen set) and replace `paginateRuns` in `metrics`/`backlog` with real cursor paging + server-side status filter; route `--dev runs list` to the dev GraphQL `runs` query.
5. Verify Phase 1 against Inngest Cloud with a real key and against the local dev server; `make check && make test`.
6. Add `runs trace <id>` (`GET /v2/runs/{id}/trace`) with text tree + JSON output and tests.
7. Add `functions invoke <app> <fn> --data/--data-file/--idempotency-key [--wait]` and `apps list|get|sync` commands with tests.
8. Add `events send` v2 path (no event key) and `api <METHOD> <path> [--body|--body-file] [--raw]` passthrough with tests.
9. Auth: accept `INNGEST_API_KEY` / `auth login --api-key` / config `api_key`, precedence flag > env > config; plaintext-HTTP credential guard for non-localhost `--api-url`; `url.PathEscape` in `GetEventRuns`; tests.
10. Root: `cmd.Context()` + `signal.NotifyContext`, retry backoff honours ctx, global `--timeout`; distinct exit codes (1 error, 2 cancelled, 4 auth) with a test that bad auth is never exit 0.
11. Small fixes: sorted keys in `-o text` map output, single `IsDevServerRunning` call in `health`, `dev runs` empty `function.id`, README/AGENTS.md config path on macOS.
12. Update CHANGELOG.md and README command reference; final `make check && make test`.
