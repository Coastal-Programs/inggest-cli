# Inngest CLI — Architecture

## Overview

Inngest CLI is a command-line tool for monitoring, debugging, and managing Inngest functions. It communicates with both the Inngest Cloud API and the local dev server, returning structured output (JSON, text, or table) suitable for AI agents, shell scripts, and CI/CD pipelines.

---

## Project Structure

```
inngest-cli/
│
├── cmd/inngest/
│   └── main.go                  # Entry point. Injects version via ldflags.
│
├── internal/
│   ├── cli/
│   │   ├── root.go              # Root Cobra command, global --env/--output/--dev flags
│   │   └── commands/
│   │       ├── auth.go          # inngest auth login/logout/status
│   │       ├── functions.go     # inngest functions list/get/config
│   │       ├── runs.go          # inngest runs list/get/cancel/replay/watch
│   │       ├── events.go        # inngest events send/get/list/types
│   │       ├── env.go           # inngest env list/use/get
│   │       ├── dev.go           # inngest dev status/functions/runs/send/invoke/events
│   │       ├── metrics.go       # inngest health/metrics/backlog
│   │       ├── apps.go          # inngest apps list/get/sync
│   │       ├── api.go           # inngest api — raw REST passthrough
│   │       ├── config.go        # inngest config show/get/set/path
│   │       └── version.go       # inngest version
│   │
│   ├── inngest/
│   │   ├── client.go            # HTTP core: auth header, retry, plaintext guard
│   │   ├── restv2.go            # REST v2 envelope/error decoding (APIError, Page)
│   │   ├── runs.go / functions.go / apps.go / environments.go / events.go
│   │   ├── devgraphql.go        # dev-server GraphQL aliased into the same types
│   │   ├── raw.go               # RawRequest for `inngest api`
│   │   └── types.go             # v2-shaped types
│   │
│   └── common/
│       └── config/
│           └── config.go        # Config load/save, env var fallbacks
│
├── pkg/
│   └── output/
│       └── output.go            # JSON / text / table output formatter
│
├── scripts/
│   └── release.sh               # Cross-platform release build
│
├── CLAUDE.md                    # AI assistant project memory and conventions
├── ARCHITECTURE.md              # This file
├── README.md                    # Public-facing documentation
├── CHANGELOG.md                 # Version history
├── Makefile                     # Build, test, release targets
├── go.mod                       # Module: github.com/Coastal-Programs/inggest-cli
└── .gitignore
```

---

## API Client Architecture

The API client (`internal/inngest/client.go`) supports three authentication modes:

| Mode | Auth Mechanism | Used For |
|------|---------------|----------|
| API key | `Authorization: Bearer <api-key>` (sent as-is) | REST API v2/v1 — recommended for CLI, CI and agents |
| Signing key | `Authorization: Bearer <sha256 of key bytes>` | REST API v2/v1 (hashed per the SDK convention; the raw key never leaves the machine) |
| Event key | Included in event payload URL | Event API (`inn.gs/e/<key>`); optional — `POST /v2/events` works without it |
| No auth | None | Local dev server (http://localhost:8288) |

Credentials are only attached to `https://` requests or loopback hosts. The `--dev` flag switches all requests to the local dev server, bypassing cloud authentication entirely.

### REST v2, v1 and the dev server

- **REST v2** (`api.inngest.com/v2/`, [OpenAPI](https://api-docs.inngest.com/api-specs/v2.json)) — runs, traces, cancel/rerun, apps, functions, invoke, envs, event schemas. Responses are `{data, page{cursor,hasMore}, metadata}`; errors `{errors:[{code,message}]}` become `*APIError`.
- **REST v1** (`api.inngest.com/v1/`) — event listing/lookup.
- **Dev Server** (`localhost:8288`) — `/api/v2/{health,envs}` plus its GraphQL API (`/v0/gql`) for runs/functions/apps, aliased into the same Go types so commands are backend-agnostic.

---

## Config System

Config file: OS config dir — `~/.config/inngest/cli.json` (Linux), `~/Library/Application Support/inngest/cli.json` (macOS) — 0600 permissions

### Environment Variable Fallbacks

| Config Key | Env Var | Description |
|-----------|---------|-------------|
| `signing_key` | `INNGEST_SIGNING_KEY` | Signing key for Cloud API |
| `event_key` | `INNGEST_EVENT_KEY` | Event key for sending events |
| _(file path)_ | `INNGEST_CLI_CONFIG` | Override config file location |

Environment variables take precedence over config file values.

### Config File Shape

```json
{
  "signing_key": "signkey-prod-...",
  "event_key": "...",
  "active_env": "production",
  "dev_server_url": "http://localhost:8288"
}
```

---

## Output System

The output formatter (`pkg/output/output.go`) supports three formats controlled by the `--output` / `-o` flag:

| Format | Description | Use Case |
|--------|-------------|----------|
| `json` | Structured JSON (default) | Piping to `jq`, AI agents, scripts |
| `text` | Human-readable key-value pairs | Quick terminal inspection |
| `table` | Tabular output with headers | Dashboard-style viewing |

Every command calls `output.Print(data, format)` — format is never hard-coded inside commands. Errors always go to stderr via `output.PrintError()`.

---

## Technology Stack

| Layer | Technology | Why |
|-------|-----------|-----|
| Language | Go 1.23 | Fast binary, easy cross-compilation, strong stdlib |
| CLI framework | Cobra v1.8.1 | Standard Go CLI framework, subcommand support |
| Output format | JSON (default) | Machine-readable for AI agent consumption |
| Config | `os.UserConfigDir()/inngest/cli.json` | Standard per-OS location, 0600 permissions |
| Build injection | Go ldflags | Embeds version at compile time |
