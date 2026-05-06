# Changelog

## [0.2.2] - 2026-05-06

### Fixed
- Release workflow: publish scoped npm packages with `--access public` (was `--access restricted`, which requires a paid npm org plan and caused E402 errors)

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
