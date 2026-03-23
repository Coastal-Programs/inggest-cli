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
│   │       ├── config.go        # inngest config show/get/set/path
│   │       └── version.go       # inngest version
│   │
│   ├── inngest/
│   │   └── client.go            # API client — GraphQL, REST, and dev server
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
| Signing key | `Authorization: Bearer signkey-...` | GraphQL API (functions, runs, environments) and REST endpoints |
| Event key | Included in event payload URL | Sending events to Inngest Cloud |
| No auth | None | Local dev server (http://localhost:8288) |

The `--dev` flag switches all requests to the local dev server, bypassing cloud authentication entirely.

### GraphQL vs REST

- **GraphQL** (`api.inngest.com/gql`) — used for querying functions, runs, environments, and metrics
- **REST** (`api.inngest.com/v1/`) — used for sending events and certain CRUD operations
- **Dev Server** (`localhost:8288/v0/`) — local API with its own schema

---

## Config System

Config file: `~/.config/inngest/cli.json` (0600 permissions)

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
| Config | `~/.config/inngest/cli.json` | Standard XDG location, 0600 permissions |
| Build injection | Go ldflags | Embeds version at compile time |
