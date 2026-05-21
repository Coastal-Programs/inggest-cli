# Changelog

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
