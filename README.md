<div align="center">
<pre>
██╗███╗   ██╗███╗   ██╗ ██████╗ ███████╗███████╗████████╗     ██████╗██╗     ██╗
██║████╗  ██║████╗  ██║██╔════╝ ██╔════╝██╔════╝╚══██╔══╝    ██╔════╝██║     ██║
██║██╔██╗ ██║██╔██╗ ██║██║  ███╗█████╗  ███████╗   ██║       ██║     ██║     ██║
██║██║╚██╗██║██║╚██╗██║██║   ██║██╔══╝  ╚════██║   ██║       ██║     ██║     ██║
██║██║ ╚████║██║ ╚████║╚██████╔╝███████╗███████║   ██║       ╚██████╗███████╗██║
╚═╝╚═╝  ╚═══╝╚═╝  ╚═══╝ ╚═════╝ ╚══════╝╚══════╝   ╚═╝        ╚═════╝╚══════╝╚═╝
</pre>

<p align="center">
  <a href="https://go.dev/">
    <img src="https://img.shields.io/badge/go-%3E%3D1.26-00ADD8.svg" alt="Go Version">
  </a>
  <a href="https://github.com/Coastal-Programs/inggest-cli/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License">
  </a>
</p>
</div>

**IMPORTANT NOTICE:**

This is an independent, unofficial command-line tool for working with Inngest's API.
This project is not affiliated with, endorsed by, or sponsored by Inngest, Inc.
"Inngest" is a registered trademark of Inngest, Inc.

> Inngest CLI for AI Agents & Automation -- a single Go binary, no runtime dependencies.

A command-line interface for monitoring, debugging, and managing [Inngest](https://www.inngest.com) functions from the terminal. Built in Go with Cobra, optimized for AI coding assistants, shell scripts, and CI/CD pipelines.

**Key Features:**
- **Single binary**: zero runtime dependencies, instant startup
- **AI-first design**: JSON output by default, structured errors, clean exit codes
- **Non-interactive**: perfect for scripts and automation
- **Flexible output**: JSON, text, or table formats
- **Cloud + local**: works with both Inngest Cloud and the local dev server
- **Real-time monitoring**: watch runs, compute metrics, check health
- **Near-zero supply chain risk**: 2 Go dependencies (cobra, pflag)

## Quick Start

### Installation

**Option 1: Go install**
```bash
go install github.com/Coastal-Programs/inggest-cli/cmd/inngest@latest
```

Requires Go 1.26+.

**Option 2: Build from source**
```bash
git clone https://github.com/Coastal-Programs/inggest-cli.git
cd inggest-cli
make build      # → ./build/inngest
make install    # → $GOPATH/bin/inngest
```

### Setup

```bash
# Authenticate with your signing key
inngest auth login

# Check auth status
inngest auth status

# List your functions
inngest functions list

# See recent runs
inngest runs list --since 1h
```

## Commands

### Authentication

```bash
# Log in with signing key (interactive prompt or --signing-key flag)
inngest auth login

# Check current auth status
inngest auth status

# Clear stored credentials
inngest auth logout
```

### Functions

| Command | Description |
|---------|-------------|
| `inngest functions list` | List all functions with triggers and config |
| `inngest functions get <slug>` | Get detailed function info by slug |
| `inngest functions config <slug>` | Show function configuration (concurrency, throttle, retry, etc.) |

```bash
# Filter by app
inngest functions list --app my-app

# Table view
inngest functions list --output table

# Full config details
inngest functions config my-app-process-order
```

### Runs

| Command | Description |
|---------|-------------|
| `inngest runs list` | List recent function runs |
| `inngest runs get <run-id>` | Get run details (status, function, timing) |
| `inngest runs cancel <run-id>` | Cancel a running function |
| `inngest runs replay <run-id>` | Replay a function run |
| `inngest runs watch` | Watch for new runs in real-time |

```bash
# Filter by status and time range
inngest runs list --status FAILED --since 1h --limit 50

# Get run details
inngest runs get 01HXYZ... --output text

# Watch runs live
inngest runs watch --function my-func --interval 5s
```

### Events

| Command | Description |
|---------|-------------|
| `inngest events send <event-name>` | Send an event to Inngest Cloud |
| `inngest events get <event-id>` | Get event details and triggered runs |
| `inngest events list` | List recent events |
| `inngest events types` | List unique event names seen recently |

```bash
# Send an event with data
inngest events send test/user.signup --data '{"userId": "123"}'

# Pipe data from stdin
echo '{"userId": "456"}' | inngest events send test/user.signup

# List recent events of a specific type
inngest events list --name user.signup
```

### Environments

| Command | Description |
|---------|-------------|
| `inngest env list` | List all environments (apps) |
| `inngest env use <name>` | Set the active environment |
| `inngest env get <name-or-id>` | Get detailed environment info |

### Dev Server

| Command | Description |
|---------|-------------|
| `inngest dev status` | Check if the local dev server is running |
| `inngest dev functions` | List functions registered with the dev server |
| `inngest dev runs` | List recent function runs from the dev server |
| `inngest dev send <event-name>` | Send an event to the dev server |
| `inngest dev invoke <function-slug>` | Invoke a function on the dev server |
| `inngest dev events` | List recent events from the dev server |

```bash
# Check dev server status
inngest dev status

# Send a test event locally
inngest dev send test/user.signup --data '{"userId": "123"}'

# Invoke a function directly
inngest dev invoke my-app-process-order --data '{"orderId": "abc"}'
```

The dev server runs at `http://localhost:8288` by default. Override with `--dev-url`.

### Monitoring

| Command | Description |
|---------|-------------|
| `inngest health` | Run connectivity and configuration health checks |
| `inngest metrics` | Show run metrics and success/failure rates |
| `inngest backlog` | Show currently queued and running runs per function |

```bash
# Health check all systems
inngest health

# Metrics for the last hour
inngest metrics --since 1h

# See what's queued
inngest backlog --output table
```

### Config

| Command | Description |
|---------|-------------|
| `inngest config show` | Show all configuration values |
| `inngest config get <key>` | Get a single configuration value |
| `inngest config set <key> <value>` | Set a configuration value |
| `inngest config path` | Print the config file path |

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--env`, `-e` | `production` | Target environment by name or ID |
| `--output`, `-o` | `json` | Output format: `json`, `text`, `table` |
| `--dev` | `false` | Route requests to local dev server |
| `--api-url` | | Override API base URL (for self-hosted Inngest) |
| `--dev-url` | | Override dev server URL |

## Output Formats

All commands support three output formats via `--output`:

- **json** (default) — structured JSON, ideal for piping to `jq` or parsing in scripts
- **text** — human-readable key-value output
- **table** — tabular output for terminal viewing

```bash
inngest functions list --output table
inngest runs list -o json | jq '.[].status'
```

## Configuration

Config is stored at `~/.config/inngest/cli.json` (0600 permissions).

### Environment Variables

| Variable | Description |
|----------|-------------|
| `INNGEST_SIGNING_KEY` | Signing key for Inngest Cloud API access |
| `INNGEST_EVENT_KEY` | Event key for sending events |
| `INNGEST_CLI_CONFIG` | Override config file path |

Environment variables take precedence over config file values.

```bash
# View all settings and their sources
inngest config show

# Set a value
inngest config set active_env staging
```

## Architecture

Built in Go with a focus on simplicity, reliability, and minimal dependencies.

- **CLI framework**: [Cobra](https://github.com/spf13/cobra) for command parsing and flag handling
- **HTTP client**: Raw `net/http` — no SDK dependency
- **API**: GraphQL for queries (functions, runs, environments), REST for events
- **Config**: Environment variables + JSON config file
- **Output**: JSON / text / table via `pkg/output.Printer`
- **Dependencies**: 2 Go modules (cobra, pflag) — near-zero supply chain risk

## Contributing

```bash
make check      # fmt + vet + test — run before every commit
make test       # run all tests with race detector
make build      # build binary to ./build/inngest
```

## License

[MIT](LICENSE)
