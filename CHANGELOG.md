# Changelog

## [0.3.0] - 2026-09-09

### Fixed
- Inngest Cloud commands were broken: the undocumented dashboard GraphQL API
  the CLI depended on had drifted (`Cannot query field "recent" on type
  "Event"`), so `runs list|get|watch`, `events list|get|types`, `metrics`
  and `backlog` failed with a 422. All Cloud calls now use the documented
  [REST API v2](https://api-docs.inngest.com) (runs, functions, apps, envs,
  invoke, trace) and v1 (events).
- A rejected credential returned `[]` with exit 0 (GraphQL answered 200 with
  an empty workspace). Bad keys now surface as a 401 `APIError` and exit 4.
- `runs watch` re-printed the same runs every tick (cursor never advanced).
- `metrics`/`backlog` paginated on `hasNextPage`, which the old client never
  set, so results were capped at 50 runs. They now follow real cursors and
  filter server-side; `backlog` is one query instead of two.
- `runs get` scanned up to 500 recent runs to find one ID; it is now a single
  request. `functions get` no longer lists every function first only to
  filter client-side by slug.
- `env list` no longer degrades to a local-config fallback on auth errors.
- `--dev` with cloud commands (`runs list --dev`) 422'd against the dev
  server; every cloud command now routes to the dev server's API under `--dev`.
- `events send --dev` without an event key posted to `/e/` and failed.
- `dev invoke` / `functions invoke --dev` posted to a non-existent dev-server
  route and got the UI's HTML back. They now use the dev server's
  `invokeFunction` mutation (what the dev UI uses) and return the real run ID.
- `events get --dev` and `events types --dev` work: the dev server lacks the
  v2 event-runs and schema endpoints, so they use its GraphQL / recent events.
- `-o text` map output had random key order.

### Added
- `runs trace <id>` — step-by-step trace tree (text) or full spans (JSON).
- `functions invoke <slug-or-id> [--data|--data-file|stdin] [--idempotency-key] [--wait]`.
- `apps list [--archived]`, `apps get <id>`, `apps sync <id> --url <serve-url>`.
- `api <path> [-X METHOD] [--body|--body-file] [--raw]` — authenticated
  passthrough to any REST endpoint (modelled on `gh api` / `inngest api`).
- `runs list`: `--app`, `--until`, `--after` cursor, `--order`, `--time-field`,
  `--output-data`; statuses/IDs accept comma-separated lists; `--since`
  accepts RFC3339 timestamps. JSON output is `{runs, page{cursor,hasMore}}`.
- `runs get --wait` / `--no-trace`; `metrics --app`; `events list --since|--after`;
  `events types --schema`; `events send --id`; `--data-file` on send/invoke.
- API-key auth: `auth login --api-key`, `INNGEST_API_KEY`, config `api_key`
  (precedence: `INNGEST_API_KEY` > `INNGEST_SIGNING_KEY` > config). API keys
  are sent as-is; signing keys stay hashed.
- Global `--timeout` (default 30s). Ctrl+C / SIGTERM now cancel in-flight
  requests and rate-limit backoff.
- Distinct exit codes: 1 error, 2 interrupted, 4 credential rejected.
- `auth status` and `health` validate the credential with an authenticated
  call instead of an anonymous GraphQL probe.

### Security
- Credentials are never sent over plaintext `http://` to a non-loopback host
  (`--api-url http://...` is refused); mirrors the official Inngest CLI.
- Response bodies are capped at 25 MB; user-supplied path segments are
  URL-escaped.

### Changed
- `runs cancel` no longer needs `--env-id`. `events send --async` (a no-op)
  was removed. `Environment` output no longer includes `slug` /
  `isAutoArchiveEnabled` (not exposed by the API); it gains `isArchived`.
- `FunctionRun` JSON: `output` is embedded JSON instead of an encoded string;
  adds `eventIDs`, `durationMs`, `appID`. Trace spans use the v2 field names
  (`id`, `stepId`, `durationMs`, `children`).
- The dev server v1.17 ignores the runs `status` filter; the CLI filters
  client-side in `--dev` mode so `backlog --dev` and `--status` are correct.

## [0.2.25] - 2026-09-09

### Security
- Bumped the `go` directive from 1.26.1 to 1.26.8, clearing 13 standard
  library vulnerabilities reported by `govulncheck` in `crypto/tls`,
  `crypto/x509` and `net/http`.

### Changed
- Updated `golang.org/x/term` 0.41.0 -> 0.46.0 and `golang.org/x/sys`
  0.42.0 -> 0.48.0.

### Added
- Test coverage for previously untested paths: `truncateBody`, `doEvent`
  header handling and body-read errors, `ListRuns` deduplication and limit
  truncation, the `CancelRun` missing-envID hint, `Save` write failures,
  `validateSigningKey` cloud-format errors, `printEnvDetail` timestamps,
  `printFunctionsTable` status precedence, and metrics truncation flags.
- `pkg/output`, `internal/inngest` and `internal/common/config` are now at
  100% statement coverage.

## [0.2.24] - 2026-05-21

### Fixed
- Release notes extraction in `release.yml` now matches the CHANGELOG
  heading exactly, so tagging `v0.2.2` no longer prefix-matches the
  `[0.2.23]` section and pulls the wrong notes.
- `scripts/release.sh` validates that the release version is semver
  before embedding it in the binary, mirroring `bump-version.sh`.
- `scripts/release.sh` parses the built binary's JSON version output with
  `python3` instead of a fragile `grep`/`sed` pipeline.

### Changed
- CI `test.yml` now runs the integration-tagged test suite
  (`go test -tags integration ./internal/cli/...`), so the version
  injection regression test runs on every push.

## [0.2.23] - 2026-05-21

### Fixed
- Release tooling: the published binary reported `version: "dev"` because
  `scripts/release.sh` and the `Makefile` built `LDFLAGS` with a fragile
  backslash-newline inside a quoted string. The `-X main.version` ldflag is
  now a single-line string, so the real release version is embedded.
- `scripts/release.sh` aborts on an empty or `dev` version and verifies
  version injection by running the freshly built host binary.
- CI `release.yml` adds a "Verify version injection" step that fails the
  job before publishing if the built binary reports a `dev`/mismatched
  version.
- Integration test `TestBinary_Version` now builds with an explicit
  `-ldflags` and asserts the reported version equals the injected value.

## [0.2.22] - 2026-05-21

### Added
- `make fix` (gofmt -w + golangci-lint --fix), `make fmt-check`, and `make hooks` targets
- `lefthook.yml` with opt-in pre-commit (gofmt + go vet) and pre-push test gate
- `.editorconfig` and `AGENTS.md` for consistent editor and agent conventions
- Coverage summary step (`go tool cover -func`) in the CI test workflow

### Changed
- `make check` is now a non-mutating gate (fmt-check + vet + lint)
- `client_test.go` retry tests use `atomic.Int64` typed counters

## [0.2.2] - 2026-05-06

### Fixed
- Release workflow: publish scoped npm packages with `--access public` (was `--access restricted`, which requires a paid npm org plan and caused E402 errors)

> **Historical note:** The `--access public` switch above applied to the 0.2.2 release only.
> The release workflow was changed back to `--access restricted` shortly afterward, and the
> npm org is now on a paid plan — so the scoped `@coastal-programs/*` packages are published
> privately. See `.github/workflows/release.yml` and `.claude/rules/release.md` for the
> current, intended behaviour.

## [0.2.1] - 2026-05-06

### Fixed
- `runs list --status` flag help now shows all-caps status values (`RUNNING,COMPLETED,FAILED,CANCELLED,QUEUED`) matching the Inngest API; was showing mixed-case which caused confusion
- Cancel confirmation prompt prints `Aborted.` when the user declines, not `Cancelled.` (which implied the run was cancelled)

## [0.2.0] - 2026-05-06

This release adds npm distribution so the CLI can be installed via `npm install -g @coastal-programs/inggest`.

### Added
- npm package `@coastal-programs/inggest` with per-platform optional sub-packages (`darwin-arm64`, `darwin-x64`, `linux-x64`, `linux-arm64`, `windows-x64`)
- `bin/inngest.js` wrapper that resolves and execs the correct platform binary
- `scripts/bump-version.sh` for atomic version sync across all 6 `package.json` files
- CI: `release.yml` now publishes all 6 npm packages (`--access restricted`) on tag push

## [Unreleased]

### Added
- Initial Inngest CLI implementation
- Auth commands: login, logout, status
- Dev server commands: status, functions, runs, send, invoke, events
- Cloud commands: functions list/get, runs list/get/cancel/replay/watch, events send/get/list
- Environment commands: list, use, get
- Monitoring commands: health, metrics, backlog
- Config commands: show, get, set, path
- JSON, text, and table output formats
- Support for Inngest Cloud and local dev server
